# Workspace I/O 示例

本示例演示如何在 `LLMAgent` 的 invocation 结束后，将 workspace 里的文件同步到调用方管理的存储中。

核心模式：`Collect` + sink 循环，写在 `AgentCallbacks` 里。框架不会把时机、错误类型、预算等打包成 sugar option——调用方自己决定在哪里写逻辑。

| API | 用途 |
| --- | --- |
| `workspaceio.WorkspaceFromContext(ctx)` | 从任意 callback 的 ctx 中解析出当前 invocation 的 workspace facade。 |
| `ws.PutFiles(ctx, files...)` | 往 workspace 写入一个或多个文件。本示例在 `BeforeAgent` 里调用只是为了演示自包含；生产环境应该通过 `codeexecutor.WorkspaceInitHook` + `InputSpec` 来同步外部 profile / skill 文件，详见 *codeexecutor* 文档。 |
| `ws.Collect(ctx, patterns...)` | invocation 结束后，按 glob 读取 workspace 文件；返回 `[]*workspaceio.File`，保留 `Truncated` 标志。pattern 语法与 `codeexecutor.WorkspaceFS.Collect` 相同（如 `out/*.json`、`runs/**/result.md`）；传单个字面路径可读单个文件，取 `result[0]`。 |
| `ws.SaveArtifact(ctx, relPath)` | 将 workspace 内的文件（`work/`、`out/`、`runs/` 下）持久化为 artifact，供后续 turn 引用。 |
| `ws.RunProgram(ctx, spec)` | 在当前 invocation 的 workspace 内执行程序，返回 `RunResult`（stdout/stderr/exit code/timed out）。等价于 `workspace_exec` LLM tool 的底层能力。 |

本示例使用 `LocalCodeExecutor`，无需 Docker 或远程沙箱即可端到端运行。"skill store" 只是主机上的 `./skills_store` 目录，生产环境替换为实际存储后端即可。

---

## 本示例做了什么

1. 配置一个 `LLMAgent`，使用 `WithCodeExecutor(localexec.New())`。
2. 在 `BeforeAgent` 回调中，从 ctx 获取 `Workspace`，预填两个 `SKILL.md` 文件（`skills/echoer/SKILL.md`、`skills/greeter/SKILL.md`）。
   - *仅限演示*：从 `BeforeAgent` 写入 workspace 只是为了让示例自包含，不需要外部存储。生产环境同样的逻辑应该放在 `codeexecutor.WorkspaceInitHook`（详见 *codeexecutor* 文档）；下面展示的读取侧模式——`Collect` + sink——完全不变。
3. 对配置的 model 发送一个简单 prompt。
4. 在 `AfterAgent` 回调中，调用 `ws.Collect(ctx, "skills/*/SKILL.md")`，遍历结果，验证每个文件，交给 `directorySink`。
5. `directorySink.Save` 将文件写入磁盘 `skills_store/<userID>/<workspace path>`。
6. 打印存储结果。

---

## 前置条件

- Go 1.21+
- 可访问的 model endpoint（示例使用 OpenAI Go SDK，支持 `OPENAI_API_KEY` 和 `OPENAI_BASE_URL`）

---

## 运行

```bash
export OPENAI_API_KEY="..."
# 可选：指向 OpenAI 兼容接口
# export OPENAI_BASE_URL="https://api.deepseek.com/v1"

cd examples/workspace_io
go run . \
  -model deepseek-v4-flash \
  -store ./skills_store \
  -prompt "Say a short hello so I can verify the agent finished."
```

预期输出（节选）：

```text
Workspace I/O demo
- model:        deepseek-v4-flash
- skill store:  /abs/path/to/examples/workspace_io/skills_store
============================================================
seeded workspace file: skills/echoer/SKILL.md (38 bytes)
seeded workspace file: skills/greeter/SKILL.md (37 bytes)
[assistant] Hello!
mirrored skills/echoer/SKILL.md -> .../skills_store/demo-user/skills/echoer/SKILL.md (38 bytes)
mirrored skills/greeter/SKILL.md -> .../skills_store/demo-user/skills/greeter/SKILL.md (37 bytes)
------------------------------------------------------------
Skill store after invocation:
- demo-user/skills/echoer/SKILL.md  (38 bytes)
- demo-user/skills/greeter/SKILL.md (37 bytes)
```

---

## 核心三行代码

```go
cb.RegisterAfterAgent(func(
    ctx context.Context, args *agent.AfterAgentArgs,
) (*agent.AfterAgentResult, error) {
    ws, ok := workspaceio.WorkspaceFromContext(ctx)
    if !ok {
        // 该 Agent 没有配置 code executor，无需同步。
        return nil, nil
    }
    files, err := ws.Collect(ctx, "skills/*/SKILL.md")
    if err != nil {
        return nil, err
    }
    for _, f := range files {
        if err := sink.Save(ctx, args.Invocation, f); err != nil {
            return nil, err
        }
    }
    return nil, nil
})
```

从 `AfterAgent` 返回 error 会让调用方感知到失败；如果想 best-effort 静默处理，可以 log 然后忽略。

---

## 适配到你的技术栈

- 替换 `directorySink`：文件只是 `*workspaceio.File`（`Path`、`Data`、`MIMEType`、`SizeBytes`、`Truncated`），可以接入数据库、对象存储、HTTP 服务等。
- 调整 `Collect` 的 pattern：语法与 `codeexecutor.WorkspaceFS.Collect` 一致（如 `out/*.json`、`runs/**/result.md`）。
- 在 sink 之前做校验：示例检查 markdown heading，你可以换成 YAML frontmatter 解析、schema 校验等。
- 如果想在每次 `workspace_exec` 后同步（而不是整个 invocation 结束后），把循环移到 `AfterTool`（按 tool name 过滤）即可。

---

## 调用方自定义策略

框架刻意保持 `Workspace` 薄层，以下几点需要你自己处理：

- **失败时跳过**：示例在 `args.Error != nil` 时直接返回，因为 workspace 状态不可靠。如果想保留部分状态用于 post-mortem，去掉 early return。
- **截断检查**：`ws.Collect` 保留后端的 `File.Truncated` 标志（单文件读取上限几 MiB）。不想静默同步半个文件，就检查这个标志。
- **容量控制**：`skills/**` 可能匹配几十 MiB 的中间状态。保持 pattern 精确，或限制 `len(files)` / `SizeBytes` 总和。
- **原子性**：`Collect` + 循环 `Save` 不是事务性的。需要 all-or-nothing 语义，先 stage 到临时目录再 rename，或使用事务性存储。
- **RunProgram 的非零退出码**：非零 `ExitCode` 通过 `RunResult` 报告，不是通过 error（符合 Go `os/exec` 约定）。检查 `result.ExitCode` / `result.TimedOut` 自行决定是否失败、重试或接受。

---

## 多节点注意事项

`workspaceio.Workspace` 与后端无关，但只操作*当前 invocation* 的 workspace。两个节点能否看到同一个 workspace 取决于 executor：

- `local` / `container`：单节点，天然隔离。
- `pcg123` + CFS：部署共享 workspace id 时可持久化。
- Cube 远程沙箱：取决于 runtime 是否暴露 stable handle。

本示例展示的模式——运行结束后 sink 到调用方管理的存储——是跨节点可见的通用做法，与后端无关。

---

## 框架源码位置

| 文件 | 内容 |
|------|------|
| `codeexecutor/workspaceio/workspace_io.go` | `Workspace`、`Collect`、`PutFiles`、`SaveArtifact`、`StageInputs`、`RunProgram` |
| `codeexecutor/workspaceio/context.go` | `WithWorkspace`、`WorkspaceFromContext` |
| `agent/llmagent/llm_agent.go` | 当 `WithCodeExecutor` 配置时，自动将 `Workspace` 安装到 invocation context |
