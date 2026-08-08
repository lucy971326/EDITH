# EDITH Studio Skills 系统设计指南

> 本文记录 edith-studio 的 Skills 系统设计决策。目标是"可行性验证过的最终方案"，不追求完美、不额外增加复杂度。
> 状态：决策已定，待实现。

## 一、产品界线：edith-studio 与 EDITH 是两个产品

| | EDITH（Web 版） | edith-studio |
|---|---|---|
| 形态 | 服务端多租户 BFF | 本地单用户 Web Coding Agent |
| 沙箱 | 对接 E2B 云沙箱 | 直接操作本地项目目录 |
| 框架利用程度 | 低，Skills / Workspace 完全自造 | 高，runner / session / summary / MCP / Skills 知识注入全用框架 |

**两个产品的框架利用程度不同，Skills 决策必须分开，互不套用。**
本文只讲 edith-studio。

## 二、决策一句话

> **用官方 `WithSkills(repo)` 的知识注入层，显式关掉执行层。**

官方 Skills 是两层独立的机器：

- **知识注入层**（渐进披露）：技能仓库解析、`skill_load` / `skill_select_docs` / `skill_list_docs` 三个知识工具、正文按需物化。EDITH 需要。
- **执行层**（沙箱跑脚本）：`skill_run` / `skill_exec` + codeexecutor + workspace 沙箱。EDITH 不需要——Studio 是 coding agent，模型靠既有 bash 工具直接执行，不需要隔离沙箱。

## 三、Skills 的心智模型：三层信息模型

1. **概览层**（极低成本）：每个技能只注入 `name + description` 到 system 的 `Available skills:` 段。模型知道"有哪些技能、各做什么"。
2. **正文层**（按需注入）：模型调 `skill_load` 后，SKILL.md 正文物化进下一次请求。
3. **文档层**（精确选择）：`skill_list_docs` 列出文档、`skill_select_docs` 选需要的。

核心机制：`skill_load` 只写一个小状态 key（`temp:skill:loaded_by_agent:*`）并返回 stub（`loaded: <name>`），**正文物化发生在每次模型请求前**——处理器读状态拼进请求。所以 Session 里只存小标记，正文在构造请求时注入，同一轮工具循环里每次模型调用都能看到已加载正文。

## 四、目录约定（三级）

与 MCP 配置的层级保持一致（`~/.edith/` 用户级、`<ProjectRoot>/.edith/` 项目级）：

| 层级 | 目录 | 内容 |
|---|---|---|
| 系统级 | `~/.edith/.system-skills/` | Studio 内置技能（软件自带）；embed 进 exe → 每次启动全量 seed 到此，系统托管 |
| 用户级 | `~/.edith/skills/` | 用户自己积累的通用技能 |
| 项目级 | `<ProjectRoot>/.edith/skills/` | 项目特有技能；位于 `.edith/`（已 gitignore，密钥不入库，不随 git 分发，与 MCP 一致） |

**系统级必须落到文件系统，不能只 embed 进二进制**：内置技能可能带 `scripts/` 脚本，脚本由 bash 工具执行，而 bash 工具需要真实文件路径。embed 只有虚拟路径，跑不了脚本。所以采用 Codex 模式：资源 embed 进 exe 随软件分发，每次启动全量 seed 到 `~/.edith/.system-skills/`（先清空再复制），用户可看可学，脚本可执行。

**系统级目录独立于用户级（`~/.edith/.system-skills` 而非 `~/.edith/skills/.system`）**：框架 `FSRepository` 用递归扫描，`.system` 若嵌套在用户级根下会被重复计入用户级，且运行时同名覆盖顺序失控。独立目录彻底避开此问题，代价只是目录名不是 `.system`。

**不做 marker 版本比对，每次启动全量替换**：系统级是系统托管命名空间，用户不该改，覆盖即重置；目录小、IO 可忽略；升级自动带新技能，零状态。

每个技能就是一个目录，含 `SKILL.md`（YAML 头 + Markdown 正文）+ 可选文档（`.md` / `.txt`）+ 可选 `scripts/` 脚本：

```
skills/
  demo-skill/
    SKILL.md        # name/description + 使用步骤 + 命令 + 输出
    USAGE.md        # 可选
    scripts/build.sh
```

## 五、后端接入（唯一的装配点）

在 `internal/workspace/workspace.go` 的 `llmagent.New(...)` 处加三行：

```go
repo, _ := skill.NewFSRepository(projectRoot, userRoot, systemRoot)

agentRuntime := llmagent.New(
    agentName,
    llmagent.WithGlobalInstruction(systemPrompt),        // 不变
    llmagent.WithSkills(repo),                            // 注册知识工具 + 概览注入 + 物化
    llmagent.WithSkillToolProfile(llmagent.SkillToolProfileKnowledgeOnly), // 关执行层，不挂 workspace_exec
    llmagent.WithSkillsLoadedContentInToolResults(true),  // 生效处 = tool result，不用 system
    llmagent.WithSkillLoadMode(llmagent.SkillLoadModeSession),             // 跨轮保留
    llmagent.WithSkillsDirectoryHints(true),              // 正文里带技能目录路径，供 bash 执行
    llmagent.WithSkillsToolingGuidance(""),               // 关默认指引（会教模型用 workspace_exec，Studio 没有）
    // ... 现有配置不动
)
```

### 说明

- **一行都不用自己写工具**：`skill_load` / `skill_select_docs` / `skill_list_docs` 由 `WithSkills(repo)` 自动注册给 LLM，LLM 自主判断调用。
- **`SkillToolProfileKnowledgeOnly` 是关键**：既关掉 `skill_run` / `skill_exec` 执行工具，也阻止框架自动挂本地 code executor（`workspace_exec`）。Studio 不需要框架的任何执行工具。
- **`WithSkillsToolingGuidance("")`**：KnowledgeOnly 下框架默认会附一段"执行脚本用 `workspace_exec`"的指引，但 Studio 没有 `workspace_exec`（脚本走 bash 工具），关掉默认指引防止误导模型。
- **根目录顺序就是覆盖优先级**：`FSRepository` 扫描多根时同名技能"先到先得"（见第六节），所以**项目级在前、用户级次之、系统级最后**。
- **不需要 `WithSkillLoads`**：见第十一节。

## 六、覆盖语义：官方原生覆盖

展示层做三级累积展示，但**运行时给模型的有效集合必须是覆盖语义**——同名技能不能有歧义。

- **运行时**：`FSRepository` 多根合并，同名先到先得 → 项目级覆盖用户级、用户级覆盖系统级。
- **展示层**：读三个目录全量列出，同名并列、各标层级，不归并。

两者并存，各司其职：**展示全量，运行覆盖**。

## 七、LoadMode 选 session

`once` / `turn`（默认）/ `session` 三档的区别是"已加载状态活多久"：

| 模式 | 正文出现在哪些请求 | 何时清空 |
|---|---|---|
| `once` | 仅下一次模型请求 | 用一次立刻清 |
| `turn` | 当前这一轮对话的所有请求 | 下一轮开始时清 |
| `session` | 之后所有轮的请求 | 手动清空 / 会话删除 |

**选 `session`**：配合侧栏手动加载（用户点一下技能名 → LLM 自主 `skill_load`），加载一次跨轮生效，之后每轮对话技能都自动在场，不用重复加载。对"项目规范"类技能尤其合适。

**注**：框架注释把 session 标为 legacy（默认是 turn），但语义符合我们的跨轮需求，实测可用，不影响决策。

**代价（已知）**：已加载技能正文本体常驻上下文，会话变长成本上升。对策是严格控制 docs 选择（不轻易 `include_all_docs`）。`WithMaxLoadedSkills` 可作为可选护栏。

## 八、生效处：tool result，绝不 system

用 `WithSkillsLoadedContentInToolResults(true)` 把已加载正文物化到 `skill_load` 的 tool result 消息上，而不是追加进 system message：

- system 前缀保持稳定 → 连续模型请求共享前缀长 → **prompt cache 命中率高**。
- 这是业界主流做法（工具消息承载动态上下文），框架实测有收益。

**已知回退**：如果对应的 `skill_load` tool result 不在当前请求的 history 里（如 history 截断），框架会回退插入一条专用 system message（`Loaded skill context:`）保证正确性。这是正确性兜底，不是常规路径。

**回退与摘要的关系（双源验证修正）**：回退不是由"会话摘要导致 tool result 缺失"触发的——恰恰相反，启用会话摘要注入时回退默认**被跳过**（`WithSkipSkillsFallbackOnSessionSummary` 默认 true），因为被 summary 掉的内容不该再塞回 prompt。只有 same-turn compaction 裁掉 tool result 时回退才重新开启。

**对 Studio 的实落影响**：Studio 已开启 `WithAddSessionSummary(true)` + `WithEnableContextCompaction(true)`，因此会话压缩产生摘要后，`session` 模式下已加载技能的正文可能暂时不在 prompt 里。降级可接受：state key 仍保留，模型可再 `skill_load` 一次恢复。如要压缩后仍保留正文，可显式 `WithSkipSkillsFallbackOnSessionSummary(false)`，代价是把已 summary 的内容重新塞回 prompt。

## 九、脚本执行：用既有 bash 工具，不用执行层

技能脚本（如 `scripts/build.sh`）由 LLM 用 Studio 现有的 **bash 工具**（`claudecode.NewToolSet`）直接执行，链路：

```
skill_load 加载 SKILL.md 正文（含目录路径，靠 DirectoryHints）
  ↓ SKILL.md 写：构建步骤 = "bash scripts/build.sh"
LLM 调用 bash 工具：bash <技能目录>/scripts/build.sh
  ↓
脚本在真实项目里执行，输出回给 LLM
```

- **`WithSkillsDirectoryHints(true)`** 让物化的正文里带 `Skill dir: <path>`，LLM 才能定位脚本。
- 与官方执行层的区别：官方把技能 staging 进隔离沙箱收集输出；Studio 的技能目录就在本地文件系统，bash 工具直接执行——跟跑 `npm test` 一样，零额外机制。
- **取舍（已知）**：官方执行层是隔离沙箱（脚本跑坏不影响宿主）；Studio 的 bash 是直接跑本机。这与项目脚本同等信任，是 Studio 既有的信任模型（本地单用户），不额外加风险。

## 十、展示层：侧栏输入框

**不做**复杂的已加载状态面板。交互：

1. 弹出 skills 列表：**系统 / 用户 / 项目**三级累积展示（同名并列、各标层级）。
2. **上下键滚动、ENTER 选中** → 技能名进入输入框（作为普通用户消息）。
3. 用户发送 → LLM 看到 `Available skills:` 概览里已存在的技能名 + 用户消息点名 → **自主判断：用户要这个 → 调 `skill_load` → 正文物化 → 开始干活**。

可选增强：列表项标注当前会话已加载状态（读 session state `temp:skill:loaded_by_agent:*`）。非必须。

**后端只需要一个接口**：`GET /api/skills`，读三个目录返回分级列表（name / description / level），供侧栏展示。不暴露密钥、不包含脚本内容。

## 十一、明确不做

- ❌ **不用官方执行层**（`skill_run` / `skill_exec` / `workspace_exec`）：脚本执行走既有 bash 工具。
- ❌ **不用 `WithSkillLoads`**：它是后端在 Run 前强制声明"本次必须加载 X"，每次 Run 都要传声明、状态不持久；且绕开模型判断。侧栏交互（用户点名 + LLM 自主 `skill_load`）已经达到同样目的，更少代码、更持久、更可控。
- ❌ **不做强制加载**（用户选中 = 立刻写 session state 强制生效）：会与 tool-result 物化机制产生回退（无对应 tool result 时落到 system message），且与"LLM 自主判断"的方案冲突。
- ❌ **不自己写物化**：`processor/skills.go` 在框架 `internal/flow/processor`，edith-studio 模块 import 不了；整条走 `WithSkills` 管线是唯一不重写处理器机制的路。
- ❌ **不实现多租户 SkillScope**（`skill` 包的 app/user 隔离）：Studio 本地单用户，用不到。

## 十二、风险与权衡

| 风险 | 对策 |
|---|---|
| `session` 模式下已加载正文常驻上下文，会话变长成本上升 | 严格 docs 选择；可选 `WithMaxLoadedSkills` 护栏；会话删除即清理 |
| `session` 正文 + 会话压缩叠加：压缩后已加载正文可能暂时从 prompt 消失（回退默认被摘要跳过） | 降级可接受：state key 保留，模型可再 `skill_load` 恢复；或显式 `WithSkipSkillsFallbackOnSessionSummary(false)`（有 token 代价） |
| 技能目录被 gitignore 保护（项目级密钥场景） | 项目级技能与 MCP 一样放 `.edith/`，密钥不入库；目录约定已统一 |
| 内置技能含脚本需要真实文件路径 | 系统级 seed 到 `~/.edith/.system-skills/`（真实路径，bash 可执行）；embed 只是分发介质，运行时物化 |
| 技能名称冲突时的语义 | 运行时覆盖（项目 > 用户 > 系统），展示层累积——决策明确，无歧义 |

## 十三、实现清单

- [x] `internal/skills` 包：读三个目录，返回分级列表（供 `GET /api/skills`）
- [x] `internal/workspace/workspace.go`：`llmagent.New` 加 `WithSkills` + KnowledgeOnly + tool-result + session + DirectoryHints + 关默认指引
- [x] `internal/studio/http.go`：加 `GET /api/skills` 路由
- [x] 前端：侧栏 skills 弹层（三级列表、上下键、ENTER 进输入框）
- [x] 系统级：内置技能 `//go:embed` + 每次启动 `SeedSystemSkills` 全量物化到 `~/.edith/.system-skills/`
- [x] 内置技能：搬入 Codex skill-creator 并适配（路径 → `~/.edith/skills`，Codex → EDITH）
- [x] 测试：仓库解析、覆盖顺序、目录不存在不报错、seed 物化与覆盖、`GET /api/skills` 返回结构
