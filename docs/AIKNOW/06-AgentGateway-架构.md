# EDITH Agent Gateway

Gateway 是 EDITH 唯一执行 Agent Run 的进程内入口。Web、飞书、GitHub App 等渠道都只是
Adapter：它们将平台消息翻译为 Gateway 请求，再自行消费 Gateway 的中性事件流。

```text
WebAdapter ─┐
IMAdapter ──┼→ Gateway → OnlyRun → ManagedRunner.Run
Scheduler ──┘              │
                            └← StreamEvent ← framework eventCh
```

## 职责

Gateway 负责将渠道事实映射为 Clerk 用户、EDITH Session 与模型，再调用 OnlyRun，并暴露状态查询与取消。
OnlyRun 在一次 Run 中依次负责：校验请求、取得同会话执行权、读取用户配置、处理图片、打开 MCP、记录 Usage、调用 Runner、完整消费 eventCh、输出中性事件、关闭 MCP 并释放执行权。

Adapter 负责 HTTP、JSON 与 SSE/Webhook；它不决定 EDITH 用户或 Session。WebAdapter 在浏览器断线后继续读完 `RunStream`，只是不再写入 SSE。
未来 IMAdapter 也读取同一个 `RunStream`，累计 `message.delta` 后一次性向平台发送最终文本。

`ManagedRunner` 是活跃 Run 的控制真相源：Gateway 的状态查询与停止均先验证 `RunStatus(requestID)` 的用户归属，再调用 `Cancel(requestID)`。SQLite `agent_runs` 只记录用量审计。

## Gateway 进程内协议

```text
Run(IncomingMessage) → RunStream{ Events <-chan StreamEvent }
RunStatus(userID, requestID) → RunStatusResponse
Cancel(userID, requestID)
```

Gateway 统一以框架流式模式执行。渠道是否展示流式内容是 Adapter 的职责，而非 Runner 的
两套执行模式：Web 转发 SSE，IM 只在结束后发送拼接好的纯文本。

流事件是渠道无关的：`run.started`、`reasoning.delta`、`message.delta`、`tool.started`、`tool.finished`、`run.completed`、`run.canceled`、`run.error`。

浏览器将它们投影为 Timeline；其他渠道自行投影为消息卡片、评论或 Check Run。渠道不直接调用 `Runner.Run`，也不拥有 MCP、Usage 或会话并发锁。

## 渠道身份与会话

`clerk_user_id` 是 EDITH 唯一用户身份。外部渠道账号必须先绑定到该用户；Gateway 不会为飞书、GitHub 或 Telegram 创建第二套用户。

```text
Web：Clerk userID → userID；浏览器 sessionId → sessionID
IM ：(channel, externalUserID) → 绑定表 → clerk_user_id
      sessionID = <channel>:<clerk_user_id>
```

Web 可以为单次消息指定模型；IM 与定时任务不传模型时，Gateway 读取该用户的默认模型。用户尚未设置默认模型时，Gateway 回退到当前 `models.DefaultModelID`。

## 代码阅读顺序

1. `gateway/`：只保留 Run、状态查询、取消的渠道门面；
2. `onlyrun/only_run.go`：一次 Run 的完整生命周期与框架事件转换；
3. `onlyrun/session_lanes.go`：同会话单实例旁路规则；
4. `onlyrun/types.go`：跨渠道执行契约；
5. `webadapter/`：Web BFF 的 HTTP/SSE 适配。
