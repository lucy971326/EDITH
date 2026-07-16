# trpc-agent-go 框架速查（EDITH 项目专用）

> **新会话提示**：读完本文件就能上手 EDITH 项目里所有 trpc-agent-go 相关代码。
> 细节去对应子文档查，再深就翻 `docs/trpc-agent-go/docs/mkdocs/zh/` 下的官方文档。

---

## 0. 一句话定位

**trpc-agent-go** = 腾讯开源的 Go 语言 Agent 框架。核心理念：
- **自主多 Agent 协作**（主推） + **工作流编排**（兼容存量）
- 统一通过 `Runner.Run()` 入口触发，自动管 Session/Plugin/事件流
- OpenAI 兼容协议接入任意 LLM（GPT / DeepSeek / Hunyuan / 自部署）

EDITH（GithubAgent）用它做：LLM 调用 → 工具执行 → AG-UI 流式输出到 Web。

---

## 1. 核心心智模型（30 秒）

```
┌─────────── HTTP 层 ───────────┐
│ AG-UI Server / A2A Server      │  ← 前端 / 其他 Agent
└─────────────┬─────────────────┘
              │ SSE / HTTP
┌─────────────▼─────────────────┐
│ Runner（执行器）                │  ← 业务代码只调它：r.Run(ctx, userID, sessionID, msg)
│  ├ Session Service             │  ← 会话/历史（Memory/Redis/SQLite…）
│  ├ Memory Service              │  ← 跨会话用户记忆
│  ├ Plugin Manager              │  ← 全局钩子（日志/安全/审批）
│  └ Agent（注册表）              │
└─────────────┬─────────────────┘
              │
┌─────────────▼─────────────────┐
│ Agent 执行单元                  │
│  ├ LLMAgent（while 循环）       │  ← 通用 LLM + 工具
│  ├ ChainAgent / ParallelAgent   │  ← 多 Agent 组合
│  └ GraphAgent（图编排）          │  ← 复杂流程、审批、暂停恢复
└─────────────┬─────────────────┘
              │
┌─────────────▼─────────────────┐
│ Flow / LLM 调用循环             │
│  ├ Model（OpenAI/Anthropic…）   │
│  ├ Tool（Function/MCP/Agent）   │
│  ├ CodeExecutor（沙箱执行）      │
│  └ Skill（三层渐进式披露）        │
└───────────────────────────────┘
```

**两个核心理解法**：
1. **接口 = 能干什么，结构体 = 需要什么**。看框架就看这两样。
2. **数据流是 `event.Event` channel**。所有执行结果都通过 `<-chan *event.Event` 异步推送。

---

## 2. 六层概念速查

| 层级 | 类型 | 关键方法 / 字段 | 子文档 |
|---|---|---|---|
| **Runner** | 接口 | `Run(ctx, userID, sessionID, msg)` | [02-Runner.md](./02-Runner.md) |
| **Agent** | 接口 + 实现 | `Run(ctx, *Invocation)` → eventChan | [03-Agent.md](./03-Agent.md) |
| **Model** | 接口 | `GenerateContent` → eventChan | (内置，无需文档) |
| **Event** | 数据结构 | 嵌入 `*model.Response`，带 InvocationID/Branch | (内置，无需文档) |
| **Tool** | 三种实现 | FunctionTool / MCP Tool / AgentTool | [04-Tool与MCP.md](./04-Tool与MCP.md) |
| **横切** | Callbacks + Plugins | Hook 点，短路 + 修改 + 覆盖 | [05-回调与插件.md](./05-回调与插件.md) |

**辅助模块**：
- [06-Session与Memory.md](./06-Session与Memory.md) — 对话历史 + 长期记忆 + 摘要
- [07-AG-UI前端协议.md](./07-AG-UI前端协议.md) — SSE 协议 + 三条路由
- [08-沙箱与Skill.md](./08-沙箱与Skill.md) — CodeExecutor + Skill 三层披露

---

## 3. 三步上手

### 3.1 最小可运行例子

```go
package main

import (
    "context"
    "fmt"
    "trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
    "trpc.group/trpc-go/trpc-agent-go/model"
    "trpc.group/trpc-go/trpc-agent-go/model/openai"
    "trpc.group/trpc-go/trpc-agent-go/runner"
)

func main() {
    // 1. 模型（OpenAI 兼容协议，DeepSeek/GPT/Hunyuan 都用这个）
    llmModel := openai.New("deepseek-chat")

    // 2. Agent
    agent := llmagent.New("assistant",
        llmagent.WithModel(llmModel),
        llmagent.WithInstruction("你是一个 GitHub 助手"),
    )

    // 3. Runner
    r := runner.NewRunner("edith-app", agent)
    defer r.Close()

    // 4. 跑
    ctx := context.Background()
    eventCh, _ := r.Run(ctx, "user-1", "session-1",
        model.NewUserMessage("你好"))

    for ev := range eventCh {
        if ev.Error != nil {
            fmt.Println("err:", ev.Error.Message)
            continue
        }
        if len(ev.Response.Choices) > 0 {
            fmt.Print(ev.Response.Choices[0].Delta.Content)
        }
        if ev.IsRunnerCompletion() {
            break  // ← 整次 Runner 结束的可靠信号
        }
    }
}
```

### 3.2 加工具（最常见的扩展）

```go
// 一个工具：读文件
readFileTool := function.NewFunctionTool(
    func(ctx context.Context, path string) (string, error) {
        data, err := os.ReadFile(path)
        return string(data), err
    },
    function.WithName("read_file"),
    function.WithDescription("读取本地文件内容"),
)

agent := llmagent.New("assistant",
    llmagent.WithModel(llmModel),
    llmagent.WithTools([]tool.Tool{readFileTool}),
)
```

### 3.3 加插件（全局拦截）

```go
import "trpc.group/trpc-go/trpc-agent-go/plugin"

r := runner.NewRunner("edith-app", agent,
    runner.WithPlugins(
        plugin.NewLogging(),                          // 日志
        plugin.NewGuardrail(),                        // 安全拦截
    ),
)
```

---

## 4. EDITH 项目当前用法

| 模块 | 用到的 trpc-agent-go 能力 | 详细位置 |
|---|---|---|
| LLM 调用 | `openai.New()` 走 OpenAI 兼容协议（DeepSeek） | `forward/` |
| Agent 编排 | `LLMAgent` + `WithSubAgents` 多 Agent | `forward/agent/` |
| 工具执行 | `FunctionTool` + `MCP ToolSet` | `forward/tools/` |
| 沙箱执行 | `CodeExecutor` 接口 + 自定义 Backend | `forward/sandbox/` |
| 前端通信 | `server/agui` + SSE | `forward/server/` |
| 会话管理 | `session.Service` + `runner.WithSessionService` | `forward/runner/` |
| 数据库 | SQLite/PostgreSQL（自实现） | `forward/store/` |

完整对接指南：[09-EDITH实战指南.md](./09-EDITH实战指南.md)

---

## 5. 关键概念答疑

### 5.1 两个"结束"信号的区别

| 信号 | 含义 | 用法 |
|---|---|---|
| `event.IsFinalResponse()` | 当前这条回复完整 | 适合流式 UI 逐字打印 |
| `event.IsRunnerCompletion()` | **整次 Runner.Run 结束** | 业务代码退出循环的唯一可靠判据 |

**记住**：99% 的场景用 `IsRunnerCompletion()`。

### 5.2 取消的正确姿势

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

eventCh, _ := r.Run(ctx, userID, sessionID, msg)

// 想停的时候：调 cancel()，然后继续读完 eventCh 防止 goroutine 泄漏
go func() {
    time.Sleep(5 * time.Second)
    cancel()
}()

for range eventCh {  // 一定要读完！
}
```

**常见错误**：只 `break` 不 cancel → goroutine 在写 channel 时阻塞 → 泄漏。

### 5.3 Partial 事件 vs 完整事件

- **Partial 事件**（`IsPartial=true`）：流式 chunk，**不写 Session**，UI 拼接显示
- **完整事件**（`IsPartial=false`）：一条完整消息，**写 Session**，可用于审计/回放

### 5.4 Tool Call vs Skill vs Code Executor

| 能力 | 适用场景 | 示例 |
|---|---|---|
| **Tool** | LLM 调用 → 执行 → 拿结果 | `read_file` / `http_get` / `mcp_tool` |
| **Skill** | 把"任务说明书"按需注入上下文 | `python-math` / `git-commit-format` |
| **CodeExecutor** | Agent 回复里夹代码块 → 自动执行 | 数据分析 / 自动化脚本 |

---

## 6. 文档索引（按需深读）

### 本项目内子文档
1. [01-核心心智模型.md](./01-核心心智模型.md) — 接口+结构体理解法
2. [02-Runner.md](./02-Runner.md) — Runner 详解 + 所有 RunOption
3. [03-Agent.md](./03-Agent.md) — LLMAgent + 多 Agent 编排
4. [04-Tool与MCP.md](./04-Tool与MCP.md) — 三种 Tool 实现
5. [05-回调与插件.md](./05-回调与插件.md) — Hook 体系
6. [06-Session与Memory.md](./06-Session与Memory.md) — 会话与记忆
7. [07-AG-UI前端协议.md](./07-AG-UI前端协议.md) — AG-UI 协议
8. [08-沙箱与Skill.md](./08-沙箱与Skill.md) — CodeExecutor + Skill
9. [09-EDITH实战指南.md](./09-EDITH实战指南.md) — EDITH 项目对接

### 框架官方文档（按主题）
> 路径前缀：`docs/trpc-agent-go/docs/mkdocs/zh/`

| 主题 | 文档 |
|---|---|
| 总入口 | `index.md` |
| 框架全景图 | `框架整体架构图.md` |
| 快速理解 | `learn/快速理解框架概念.md` |
| Agent（含 LLMAgent / Invocation / Callbacks） | `agent.md` |
| Runner（含 RunOption / Ralph Loop / Best-of-N） | `runner.md` |
| 多 Agent（Chain/Parallel/Cycle/Team/Swarm） | `multiagent.md` |
| Graph（图编排、HITL） | `graph.md` |
| Tool / FunctionTool / StreamableTool | `tool.md` |
| MCP | `tool.md`（内嵌 MCP 章节） |
| Callbacks 四类 | `callbacks.md` |
| Plugin 插件 | `plugin.md` |
| Session（含 8 种后端） | `session.md` + `session/<backend>.md` |
| 摘要增量机制 | `session/summary.md` |
| Memory（Agentic / Auto） | `memory.md` |
| Artifact | `artifact.md` |
| Skill（三层披露） | `skill.md` |
| CodeExecutor / Workspace | `codeexecutor.md` |
| CodeAct（Agent 自主写代码） | `codeact.md` |
| Planner | `planner.md` |
| Knowledge / RAG | `knowledge/index.md` |
| Event 详解 | `event.md` |
| Model / Provider 抽象 | `model.md` |
| Observability / OTel | `observability.md` |
| AG-UI 协议 | `agui/index.md` |
| A2A 协议 | `a2a.md` |
| Prompt / Late Context | `prompt.md` |
| Error Handling | `error-handling.md` |
| Evolution（反思） | `evolution.md` |
| Evaluation（评估） | `evaluation.md` |

### 框架源码
- 路径：`docs/trpc-agent-go/`
- 查询工具：codegraph MCP（`mcp__codegraph__codegraph_explore`）
- **优先级**：官方文档 > 源码注释 > 源码实现

---

## 7. 常见速记

| 想做什么 | 调什么 |
|---|---|
| 加 LLM | `openai.New(modelName)` |
| 加 Agent | `llmagent.New(name, WithModel(...), WithTools(...))` |
| 加工具 | `function.NewFunctionTool(fn, WithName(...))` |
| 加 MCP | `mcp.NewMCPToolSet(mcp.ConnectionConfig{...})` |
| 跑对话 | `runner.Run(ctx, userID, sessionID, msg)` |
| 接前端 | `server/agui.New(runner, agui.WithPath("/chat"))` |
| 持久化会话 | `runner.WithSessionService(sqlite.NewService(...))` |
| 加全局拦截 | `runner.WithPlugins(...)` |
| 加按 Agent 拦截 | `llmagent.WithAgentCallbacks(...)` |
| 看执行链路 | `agent.WithExecutionTraceEnabled(true)` |
| 加 Skill | `llmagent.WithSkills(skill.NewFSRepository(...))` |
| 加沙箱 | `llmagent.WithCodeExecutor(myExecutor)` 或 `agent.WithCodeExecutor(...)` 单次 |

---

**下一步**：从 [01-核心心智模型.md](./01-核心心智模型.md) 开始深入，或者直接跳到 [09-EDITH实战指南.md](./09-EDITH实战指南.md) 看你关心的部分。
