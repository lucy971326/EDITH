# 默认 AG-UI 服务端

此示例暴露了一个由 `tRPC-Agent-Go` 运行器（runner）支持的最小化 AG-UI SSE 端点。

它旨在与 [Copilotkit 客户端](../../client/copilotkit/) 配合使用。

## 运行

在 `examples/agui` 模块中运行：

```bash
# 在 http://localhost:8080/agui 启动服务端
go run .
```

服务端会打印包含绑定地址的启动日志：

```
2025-09-26T10:28:46+08:00       INFO    default/main.go:60      AG-UI: serving agent "agui-agent" on http://127.0.0.1:8080/agui
```

## 前端分组

如果你的前端需要按照子 Agent 分组展示流式传输的工具调用，请在服务端启用源元数据（source metadata）：

```go
server, err := agui.New(
    runner,
    agui.WithEventSourceMetadataEnabled(true),
)
```

启用后，转换后的 AG-UI 事件将携带一个紧凑的 `rawEvent` 对象。典型字段包括：

- `eventId`：原始的 `trpc-agent-go` 事件标识符
- `author`：触发原始事件的 Agent
- `invocationId`：产生该事件的具体调用（invocation）
- `parentInvocationId`：当事件来自嵌套的子 Agent 时，父级调用的 ID
- `branch`：执行分支，当同一个 Agent 在单次请求中多次运行时非常有用

`rawEvent` 是可选的。它仅出现在由 AG-UI 转换器（translator）或 AG-UI 消息快照构建器（message snapshot builder）生成的 AG-UI 事件上。当框架没有非空的源元数据可暴露时，该字段会被省略。

推荐的分组策略：

- 使用 `rawEvent.author` 为每个 Agent 名称显示一个分组（bucket）。
- 使用 `rawEvent.branch` 为每次具体的子 Agent 执行显示一个分组（bucket）。
