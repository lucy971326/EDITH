# 08 - 沙箱（CodeExecutor）与 Skill

> CodeExecutor = 让 Agent 在隔离环境执行代码。Skill = 把"任务说明书"按需注入上下文。

---

## 1. CodeExecutor 心智模型

> CodeExecutor = 代码执行的核心接口。让 Agent 具备数据分析、科学计算、脚本自动化的实际工作能力。

---

## 2. 核心接口

```go
type CodeExecutor interface {
    ExecuteCode(context.Context, CodeExecutionInput) (CodeExecutionResult, error)
    CodeBlockDelimiter() CodeBlockDelimiter
}

type CodeExecutionInput struct {
    CodeBlocks  []CodeBlock
    ExecutionID string
}

type CodeExecutionResult struct {
    Output      string
    OutputFiles []File
}
```

**两个内置实现**（加上容器）：
- `local.New(...)` — 本地直接执行（开发测试）
- `container.New(...)` — Docker 容器隔离（生产推荐）
- 远程后端（E2B/远程容器）需要自实现

---

## 3. Workspace（核心概念）

新版 CodeExecutor 引入 **Workspace** 抽象，统一管理工作区目录：

```
Workspace
  ├─ out/        ← 产物（写完后公开给后续轮次）
  ├─ work/       ← 临时工作（每轮清空）
  ├─ inputs/     ← 用户上传/外部输入
  └─ skills/     ← Skill 文件（容器只读挂载）
```

完整说明：`codeexecutor.md#workspace-中有哪些目录`

**关键规则**：
- `out/` 里的文件**会保留**到下一轮（用 `*ArtifactRef` + `WithSaveArtifactMaxBytes` 配置）
- `work/` 里的文件下一轮被清空
- 用户上传的文件出现在 `inputs/`

---

## 4. 怎么选后端

| 后端 | 适用 | 隔离 | 速度 |
|---|---|---|---|
| **Local** | 开发测试 | ❌ 无隔离 | 最快 |
| **Container（Docker）** | 生产 | ✅ 容器隔离 | 启动慢 |
| **Remote（自实现）** | 云端沙箱（E2B） | ✅ 完全隔离 | 视网络 |

详见 `codeexecutor.md#怎么选后端`

---

## 5. Workspace Backend 接口（EDITH 重点）

如果你要实现远程沙箱（如 E2B），核心是实现 Backend：

```go
type Backend interface {
    // 执行命令
    RunProgram(ctx context.Context, cmd *Command) (*Result, error)

    // 文件读写
    Read(ctx context.Context, path string) ([]byte, error)
    Write(ctx context.Context, path string, data []byte) error

    // 文件列举
    Ls(ctx context.Context, dir string) ([]FileInfo, error)
    Exists(ctx context.Context, path string) (bool, error)

    // 关闭
    Close() error
}
```

详见 `codeexecutor.md` 中：
- `应用代码访问 workspace`：讲解怎么在 callback 里读写
- `RunProgram 怎么写 Cmd / Args`：执行命令的细节
- `*File`：返回文件结构
- `*ArtifactRef + WithSaveArtifactMaxBytes`：把 workspace 产物公开给后续轮次

---

## 6. 用户上传文件到工作区

两种方式：

```go
// 方式 1：直接把文件内容放进消息
fileMsg := model.NewUserMessage(
    "请分析这个 CSV",
    model.NewFilePart("data.csv", csvData, "text/csv"),
)

// 方式 2：先上传到 artifact，再在消息里只放引用
fileID := toolCtx.SaveArtifact("data.csv", artifact)
// 然后告诉 Agent 文件 ID，Agent 在 workspace 里引用
```

详见 `codeexecutor.md#用户上传的文件会出现在哪里`

---

## 7. 限制 `workspace_exec` 可执行的命令

```go
containerExec := container.New(
    container.WithAllowedCommands(
        regexp.MustCompile(`^python3? .*`),
        regexp.MustCompile(`^bash .*`),
    ),
)
```

**匹配规则**：白名单正则列表，任一匹配即放行。

**会被拒掉的写法**（默认）：`rm -rf /`、`curl ... | bash`、反弹 shell 等。

详见 `codeexecutor.md#限制-workspace_exec-可执行的命令`

---

## 8. Workspace Init Hooks

```go
containerExec := container.New(
    container.WithInitHook(func(ctx context.Context, ws Workspace) error {
        // 每次新建 workspace 时执行：装依赖、预下载数据等
        return ws.Run(ctx, &Command{Cmd: "pip", Args: []string{"install", "numpy"}})
    }),
)
```

详见 `codeexecutor.md#workspace-init-hooks`

---

## 9. 与 Skill 的关系

Workspace 自动包含 Skill 文件（容器只读挂载），Skill 工具（`workspace_exec` 等）可在 workspace 里运行 Skill 脚本。

详见 `codeexecutor.md#与-skill_load-的关系`

---

## 10. 环境变量与执行环境

执行时注入：`WORKSPACE_DIR` / `SKILLS_DIR` / `WORK_DIR` / `OUTPUT_DIR`

详见 `codeexecutor.md#环境变量与执行环境`

---

## 11. Skill（技能）

### 11.1 心智模型

> Skill = 可复用的任务说明书 —— `SKILL.md` 描述目标与流程，加载正文后在工作区中执行脚本。
> 三层信息模型：概览（低成本）→ 正文（按需注入）→ 文档/脚本（精确选择 + 隔离执行）。

### 11.2 三层渐进式披露

```
第 1 层：概览（每次请求）
  System Prompt 一行：Available skills:
    - python-math: Math utilities
    - ocr: Image text extraction
  成本：极低
  方式：injectOverview() 自动注入

第 2 层：正文（按需注入）
  模型调用 skill_load("python-math")
  → 写入 session state temp key
  → 下次请求把 SKILL.md 正文物化到 Prompt
  成本：只有被加载的技能才占 token

第 3 层：文档/脚本（精确选择 + 隔离执行）
  skill_select_docs → 只选必要文档
  workspace_exec → 在 /skills/<name>/ 工作区里执行脚本
  脚本从不内联到 Prompt，在工作区隔离执行
```

### 11.3 Repository 接口

```go
type Repository interface {
    Summaries() []Summary              // 所有技能摘要
    Get(name string) (*Skill, error)   // 完整内容
    Path(name string) (string, error)  // 技能目录路径
}
```

### 11.4 FSRepository（本地文件系统实现）

```go
import "trpc.group/trpc-go/trpc-agent-go/skill"

repo, _ := skill.NewFSRepository("./skills")
repo.Refresh()  // 文件变动后重新扫描
```

**SKILL.md 格式**：
```markdown
---
name: python-math
description: Small Python utilities for math and text files.
---

Run short Python scripts inside the skill workspace...

## Examples
1) Print Fibonacci numbers
   Command: python3 scripts/fib.py 10 > out/fib.txt

## Output Files
- out/fib.txt
```

### 11.5 三个内置工具

| 工具 | 用途 |
|---|---|
| `skill_load` | 加载技能正文 + 可选文档 |
| `skill_select_docs` | 增/删/改文档选择 |
| `skill_list_docs` | 列出技能可用文档 |

### 11.6 SkillLoadMode（内容驻留多久）

```go
const (
    SkillLoadModeOnce    = "once"    // 只在下次模型请求注入一次
    SkillLoadModeTurn    = "turn"    // 默认：当前 Run 内所有请求可见
    SkillLoadModeSession = "session" // 跨多轮对话保留
)
```

| 模式 | 生命周期 | 适用 |
|---|---|---|
| `once` | 下一次请求 | 一次性查询 |
| `turn`（默认） | 当前 Run | 最小权限 |
| `session` | 跨多轮 | 反复用到 |

### 11.7 注入位置

```go
llmAgent := llmagent.New("assistant",
    llmagent.WithSkills(repo),
    // llmagent.WithSkillsLoadedContentInToolResults(true),  // 改注入到 tool result
)
```

| 位置 | 优点 | 缺点 |
|---|---|---|
| System Prompt（默认） | 永远在 | System 变化，Prompt Cache 变短 |
| Tool Result | System 稳定，缓存命中率高 | 截断时需回退插入 System |

### 11.8 注册到 Agent

```go
llmAgent := llmagent.New("assistant",
    llmagent.WithSkills(repo),
    // 自动注册 3 个 skill_* 工具
)
```

**自动 executor fallback**：`WithSkills(repo)` 没显式配 executor 时自动注入本地 executor。

---

## 12. EDITH 集成沙箱的思路（远程 E2B）

E2B 远程沙箱不是 trpc-agent-go 内置的，需要自实现：

1. **实现 `Backend` 接口**（参见第 5 节），包装 E2B SDK
2. **包装成 `CodeExecutor`**，实现 `ExecuteCode` + `CodeBlockDelimiter`
3. **可选：实现 Workspace init hooks**（首次连接时 `pip install` 之类）
4. **可选：实现 `WithAllowedCommands`**（命令白名单）
5. **通过 `llmagent.WithCodeExecutor(myE2BExec)` 注入**

⚠️ **Skill 文件不自动复制**：框架默认假设技能文件在同一台机器。如果 Skill 脚本需要在 E2B 沙箱执行，需要在 Workspace init hook 里手动上传。

---

## 13. CodeAct 模式

`codeact.md` 文档介绍了一种特殊模式：Agent **自主写代码并执行**，而不是调用预定义工具。

完整文档：`docs/trpc-agent-go/docs/mkdocs/zh/codeact.md`

---

## 14. 踩坑提醒

| 坑 | 解法 |
|---|---|
| `work/` 文件下一轮没了 | 用 `out/` 持久化产物 |
| 容器启动慢 | 用 init hooks 预装依赖 + 持久容器 |
| 远程沙箱 Skill 脚本找不到 | 在 init hook 里手动上传 skill 目录 |
| `workspace_exec` 被拒 | 检查 `WithAllowedCommands` 白名单 |
| 用户上传的文件太大 | 用 `WithSaveArtifactMaxBytes` 限制 |
| Skill 概览太多 → System Prompt 变长 | `WithMaxOverviewSkills(N)` 限制 |

---

## 15. 去哪查

- **CodeExecutor 总览**：`docs/trpc-agent-go/docs/mkdocs/zh/codeexecutor.md`
  - Workspace 目录：`codeexecutor.md#workspace-中有哪些目录`
  - 怎么选后端：`codeexecutor.md#怎么选后端`
  - Backend 接口与 RunProgram：`codeexecutor.md#应用代码访问-workspace`
  - 用户上传文件：`codeexecutor.md#用户上传的文件会出现在哪里`
  - 命令白名单：`codeexecutor.md#限制-workspace_exec-可执行的命令`
  - Init hooks：`codeexecutor.md#workspace-init-hooks`
  - 环境变量：`codeexecutor.md#环境变量与执行环境`
- **CodeAct 模式**：`docs/trpc-agent-go/docs/mkdocs/zh/codeact.md`
- **Skill 完整**：`docs/trpc-agent-go/docs/mkdocs/zh/skill.md`
  - 三层披露：`skill.md#核心概念：三层信息模型`
  - SKILL.md 格式：`skill.md#skill_md-结构与示例`
  - 工具用法：`skill.md#工具用法详解`
  - 故障排查：`skill.md#故障排查`
