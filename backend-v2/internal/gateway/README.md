# gateway

Gateway 只翻译渠道身份和会话，不加载 Agent 配置。

```text
IncomingMessage
      │
      ├─ web / cron：ExternalUserID 已是 Clerk UserID
      │
      └─ IM：Bindings.ToClerkUserID
      │
      ▼
agentrun.Request
      │
      ▼
AgentRun.Run
```

默认模型、MCP、图片和用户人格全部由 AgentRun 聚合，不属于 Gateway。
