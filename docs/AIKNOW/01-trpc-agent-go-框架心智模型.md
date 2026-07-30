# tRPC-Agent-Go：EDITH 只需要知道的框架心智模型

> 目标：让接手 EDITH 的人快速理解“框架负责什么、一次请求如何运行、哪些能力 EDITH 不采用”。
> 
> 这不是 API 手册。遇到具体字段、回调顺序或接口签名，查 `docs/learn/trpc-agent-go/`、官方文档或源码。

---

## 1. 一句话模型

**tRPC-Agent-Go 是一个把“消息 + 本次运行配置”执行成“事件流”的 Agent 运行框架。**

EDITH 使用它做：模型调用、LLM 工具循环、Session/Memory、事件流和 RunOptions 覆盖。

EDITH 不使用它做：Sandbox/Workspace、Artifact、Skill Repository/`skill_load`。

```text
客户端 / BFF
  → Go Runtime: Runner.Run(ctx, userID, sessionID, message, opts...)
  → Agent: 调模型、决定是否调用工具、继续推理
  → Event Channel: 连续事件
  → SSE / IM 适配层
```

## 2. 五个核心概念

| 概念 | 它是什么 | EDITH 怎样用 |
|---|---|---|
| `Runner` | 一次运行的唯一入口和协调者 | 进程级仅一个，所有用户、所有渠道共享 |
| `LLMAgent` | LLM + 工具调用循环 | 长期复用的“零默认”骨架，不保存用户配置 |
| `model.Model` | 对某个模型供应商的调用能力 | 进程级复用；不保存用户 API Key |
| `tool.Tool` | 给模型调用的能力声明与执行实现 | 默认工具长期注册；用户 MCP / Sandbox 工具按 Run 追加 |
| `event.Event` | 运行期间所有输出的统一载体 | HTTP SSE、未来 IM 都消费同一条事件流 |

`Runner` 是服务端思维的中心，不是 `Agent`。不要在 HTTP handler 中创建 Agent，也不要为每个用户保留一个 Agent 实例。

## 3. 一次 Run 到底发生了什么

```text
1. EDITH 读取该用户的运行配置
2. EDITH 组装 RunOptions（模型、Prompt、动态工具等）
3. Runner 用 <appName, userID, sessionID> 读取 Session
4. Agent 将 system prompt、历史、当前消息、工具声明交给模型
5. 模型回复文本，或请求工具
6. 框架执行工具、将结果写回模型上下文，直到结束
7. Runner 持久化需要保存的 Session 事件，并发送 runner completion 事件
```

这意味着框架会自动维护**对话历史**；EDITH 不应把完整历史再通过 `RunOptions.Messages` 手工塞入。

## 4. Event 流：输出与结束判断

`Runner.Run` 返回 `<-chan *event.Event`。前端不是在等一个字符串，而是在消费一条运行事件流。

- 流式文本：读取 `Choice.Delta.Content`。
- 工具调用、工具结果、错误也会以事件出现。
- Usage 要在循环中持续记录最后一个非空值。
- **整次 Run 是否结束，唯一可靠判断是 `ev.IsRunnerCompletion()`。**
- 收到结束事件后仍应确保 channel 被消费、资源被收尾。

流式输出与模型 API 流式是两个层次：模型通常持续流式；`WithStream` 决定 Run 对 handler 的事件输出是否分段。

## 5. Session 和 Memory 的边界

| 数据 | 隔离键 | 用途 |
|---|---|---|
| Session | `<appName, userID, sessionID>` | 一段对话的历史和状态 |
| Memory | `<appName, userID>` | 跨会话的用户偏好、长期事实 |

EDITH 的 `appName` 固定为 `edith`。用户隔离的根是可信的 `userID`，会话隔离的根是 `sessionID`。

## 6. 工具模型

框架只要求工具有统一的声明和调用方式；工具来自两层：

```text
LLMAgent 长期工具：所有用户都一样的系统工具
RunOptions.AdditionalTools：本用户、本次运行专属工具
```

EDITH 将用户 MCP 工具放在第二层。Sandbox ToolSet 作为长期默认工具注册，但每次调用时从
Invocation ctx 取得当前 `userID + sessionID`，因此实际操作的仍是本次用户自己的 sandbox；
它不保存任何用户状态。

MCP ToolSet 有连接生命周期：每个 Run 创建的 ToolSet 必须在**该 Run 的事件流消费完成后**关闭。不能在加载函数返回前关闭，也不能依赖 `Runner.Close()` 代管。

## 7. 取消模型

`context.Context` 是任务调用链的取消信号。EDITH 的 Web HTTP 请求和 Agent 任务使用不同的 ctx：浏览器断线只停止 SSE 推送；任务 ctx 继续传给 Runner、模型 HTTP 请求、MCP HTTP 请求和 EDITH 自己实现的工具。

每个 Run 都有 `RequestID`。EDITH 已通过 `ManagedRunner` 用它实现用户主动停止和活跃
状态查询：`RunStatus(requestID)` 存在即仍在执行，`Cancel(requestID)` 发送取消信号。
RunStatus 不存在表示 Runner 已不再管理该 Run；浏览器随后从 Session 历史恢复 Timeline。
配额和运维中止将复用同一取消入口。任何长耗时 EDITH 工具都必须尊重任务 ctx 的
`Done()`。

## 8. 框架边界：不要误用本地 Agent 能力

框架原生的 `CodeExecutor`、Workspace、Artifact 和 Skill Repository 更适合本地 Agent 工作区模型。EDITH 1.0 明确不用它们：

| 能力 | EDITH 归属 |
|---|---|
| 沙箱创建 / pause / reconnect / Volume | E2B SDK + EDITH Sandbox 服务 |
| 用户 Skill Markdown 与 scripts | 将来的 EDITH Skills 系统；不使用框架 Skills |
| 上传、临时文件、产物 | E2B sandbox 文件系统 + EDITH 自己的规则 |
| 给模型执行沙箱能力 | EDITH 自己的 sandbox tools，按 Run 注入 |

不要为了“接入框架”再把这些能力包回 `CodeExecutor`。框架只负责调度 EDITH 提供的工具。

> 现状：Sandbox 已完成；EDITH 自有 Skills 尚未实现，下一阶段才设计。不要把旧研究中的
> “E2B Volume 存 Skills”当成已经确定的实现。当前完成状态以 `06-会话接力.md` 为准。

## 9. 接手时的检查清单

- 是否仍然只有一个长期 `Runner` 和一个长期 `LLMAgent`？
- 是否所有用户差异都在每次 `Runner.Run(..., opts...)` 中装配？
- 是否 API Key 仅随请求 Header 进入模型，而未写进共享 Model？
- 是否用户 MCP 工具只通过 `AdditionalTools` 出现？Sandbox 工具是否只从 Invocation 解析本次会话？
- 是否按 `<userID, sessionID>` 管理沙箱？
- 是否以 `IsRunnerCompletion()` 判断结束，并在随后关闭本次 MCP 资源？
