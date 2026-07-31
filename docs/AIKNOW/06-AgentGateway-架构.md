# EDITH Agent Gateway

Gateway 是 EDITH 唯一执行 Agent Run 的入口。它借鉴 OpenClaw 的双入口设计：HTTP 客户端走 Gateway 路由，飞书、GitHub App 等渠道未来直接调用同一个 Go 方法。

```text
Web BFF HTTP ─┐
飞书 / GitHub ├→ Gateway → ManagedRunner.Run
              └← Gateway 按渠道输出 ← framework eventCh
```

## 职责

Gateway 在一次 Run 中依次负责：校验请求、取得同会话执行权、读取用户配置、处理图片、打开 MCP、记录 Usage、调用 Runner、完整消费 eventCh、发送中性事件、关闭 MCP 并释放执行权。

`ManagedRunner` 是活跃 Run 的控制真相源：Gateway 的状态查询与停止均先验证 `RunStatus(requestID)` 的用户归属，再调用 `Cancel(requestID)`。SQLite `agent_runs` 只记录用量审计。

## 协议

```text
POST /internal/gateway/messages:stream
GET  /internal/gateway/runs/{requestID}
POST /internal/gateway/runs/{requestID}/cancel
```

流事件是渠道无关的：`run.started`、`reasoning.delta`、`message.delta`、`tool.started`、`tool.finished`、`run.completed`、`run.canceled`、`run.error`。

浏览器将它们投影为 Timeline；其他渠道自行投影为消息卡片、评论或 Check Run。渠道不直接调用 `Runner.Run`，也不拥有 MCP、Usage 或会话并发锁。

## 代码阅读顺序

1. `gateway/server.go`：长期能力与路由；
2. `gateway/message.go`：一次 Run 的完整生命周期与 SSE 输出；
3. `gateway/session_lanes.go`：同会话单实例规则；
4. `gateway/status.go`、`gateway/cancel.go`：ManagedRunner 控制面；
5. `gateway/types.go`：跨渠道契约。
