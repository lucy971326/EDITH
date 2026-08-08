# EDITH Studio BUG / 问题记录

> 本文件记录真实测试中发现的问题。每条含现象、根因、改法方向、优先级，供后续修复决策。
> 2026-08-08：模型用 skill-creator 技能创建 novel-writing 时的真实日志 + 模型自述 14 条问题。

---

## 一、问题清单（按类别）

### A. 环境层面（WSL / Windows 混用）

| # | 问题 | 现象 | 根因 | 改法方向 |
|---|---|---|---|---|
| A1 | bash 跑在 WSL，stderr 全程被污染 | 每次命令 stderr 混入 UTF-16 乱码警告（`wsl: 检测到本地主机...`）；`ls` 首次 exitCode=2 | claudecode bash 工具硬编码 `bash`（bash.go:43），Windows PATH 里 `bash` = WSL 启动器 | 平台分支：Windows 显式指向 Git Bash，macOS/Linux 走 `/bin/bash` |
| A2 | 路径两套体系 | 工具链 `C:\...` vs bash 里 `/mnt/c/...`，手动转换易错 | bash 走 WSL，挂载盘路径语义不同 | 随 A1 消解 |
| A3 | 中文不敢用 bash 写 | 中文字符过 WSL 有编码风险，只能靠编辑器写 | WSL 编码传递不可靠 | 随 A1 消解 |

### B. 工具链限制（影响最大）

| # | 问题 | 现象 | 根因 | 改法方向 |
|---|---|---|---|---|
| B4 | Read/Write/Edit/Glob/Grep 全锁项目目录 | 访问 `~/.edith/...`（用户/系统 skills）一律 `path is outside base_dir` | claudecode `WithBaseDir(projectRoot)` 安全边界 vs 技能存储位置冲突 | **定案（方案 c）**：本地 copy claudecode 到 `internal/`，`normalizePath` 删越界分支放开全盘；`tool/file` 写死单根、`WithReadOnlyDirs` 只扩读，走不通 |
| B5 | Write 的 stale 保护死锁 | Write 要求"先 Read 过"，但文件在 base_dir 外 Read 不了 → 永远写不了 | file_state 先读后写 + base_dir 锁叠加 | 随 B4 消解 |
| B6 | Glob 对不存在目录报错 | `Directory does not exist` 而非空结果，误导成"没权限" | 框架对不存在路径的处理 | 轻微，可顺手改 |

### C. skill-creator 脚本缺陷（值得开发修复）

| # | 问题 | 现象 | 根因 | 改法方向 |
|---|---|---|---|---|
| C7 | `$skill-name` 被吞（最典型） | `--interface default_prompt="Use \$skill-name ..."` → 生成 `Use -name`；换单引号仍 `Use -name` | Windows→WSL 参数传递层把 `$skill` 当变量展开成空（A1 衍生） | 随 A1 消解；且脚本应对 default_prompt 走字面值通道或生成后自检 |
| C8 | quick_validate.py 不校验 openai.yaml | C7 的坏 yaml 验证器抓不住，人眼发现 | 验证器只查 SKILL.md frontmatter | 扩展验证器：校验 openai.yaml 结构 + default_prompt 必须含 `$skill-name` |
| C9 | generate_openai_yaml.py 写 CRLF | cat 能看到 `\r\n`，与 SKILL.md LF 混用，git diff 噪音 | WSL 挂载盘文本模式行尾转换（A1 衍生） | 随 A1 消解；脚本强制 LF |

### D. 流程 / 文档层面

| # | 问题 | 现象 | 根因 | 改法方向 |
|---|---|---|---|---|
| D10 | 三套 skill roots 并存、发现机制不透明 | s1(项目)/s2(用户)/s3(系统)；s2 启动时不存在；s2 与 base_dir 冲突；模型最终 mv 进 s1 并删 s2 副本 | 三级目录设计 + 框架对不存在目录返回空不自动建 | 启动时自动 mkdir 用户/项目级 skills 目录；侧栏/文档讲清三级关系 |
| D11 | skill-creator 指南没提项目级选项 | 默认建 `~/.edith/skills`，对项目内工作不是最优 | 从 Codex 搬来未适配 EDITH 三级 | SKILL.md 补"项目内工作建议用项目级 `.edith/skills`" |
| D12 | init_skill.py 生成英文 TODO 模板 | 中文技能整篇重写 | 模板只有英文（预期行为） | 可选：补中文模板 |

### E. 其他小坑

| # | 问题 | 现象 | 改法方向 |
|---|---|---|---|
| E13 | 路径含空格/特殊字符时 bash 引号易错 | — | 使用方注意，脚本侧可用列表参数 |
| E14 | 乱码混入后难分辨"命令报错"和"WSL 噪音" | 第一反应差点误判 | 随 A1 消解 |

---

## 二、根因归并（14 条 → 3 个根源）

1. **bash 工具落 WSL**（A1/A2/A3/C7/C9/E14）：claudecode 硬编码 `bash`，Windows PATH 指向 WSL 启动器。统一 shell 可连带消解约 6 条。
2. **文件工具 base_dir 锁死**（B4/B5/B6）：安全边界（项目目录）与技能存储位置（`~/.edith`）冲突。最疼，卡死 skill-creator 完整流程。
3. **skill-creator 脚本 + 文档适配**（C8/D10/D11/D12）：验证器不查 openai.yaml、指南未适配 EDITH 三级、用户/项目目录不自动建。

---

## 三、优先级

| 优先级 | 修什么 | 为什么 |
|---|---|---|
| **P0** | B4 文件工具多目录支持 | 最疼，skill-creator 读参考→编辑→校验被卡死 |
| **P0** | A1 统一 bash shell（Windows 分支指 Git Bash） | 一个修复连带消掉 A2/A3/C7/C9/E14 六条 |
| **P1** | C7 脚本自检 + C8 验证器扩展 | 防坏技能无声通过 |
| **P1** | D11 SKILL.md 补三级说明 + D10 自动建目录 | 模型第一次上手就不懵 |
| **P2** | B6 / D12 / E13 | 打磨 |

---

## 四、关键判断：框架定位与 EDITH 选型错位

**A1 / B4 暴露的不是框架"烂"，而是定位不匹配：**

- trpc-agent-go 是**服务端 Agent 框架**（runner / session / summary / 多租户是强项，Linux 优先）。
- claudecode 工具集（bash / 文件 / grep）是**兼容 Claude Code 的本机工具包，附加物**，Windows 支持薄弱。
- EDITH 是**本地单用户桌面产品**：框架强项在主力用（很稳），框架弱项（本机 bash / 文件工具）恰好是 EDITH 的命根子。

**结论**：文件工具 base_dir 锁（B4）和 bash shell 选择（A1）属于"本机工具"范畴，是 EDITH 的地盘而非框架核心。**长期看，本机工具层 EDITH 可能要自己造**，而不是在框架上打补丁。短期（开发期）可用 fork + `go.mod replace` 或环境适配先顶住。

**A1 跨平台说明**：改法必须是平台分支（Windows 指 Git Bash，macOS/Linux 走 `/bin/bash`），不影响 Mac。Mac 另注意自带 bash 3.2（缺关联数组、`**` 等 bash4+ 特性），skill 脚本若用到需用 brew bash / zsh。
