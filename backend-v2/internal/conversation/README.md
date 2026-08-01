# conversation

将框架保存的会话事件投影为 Web Timeline。

```text
Session Service ──► history.go ──► timeline_projection.go ──► Timeline
                         │                 │
                         │                 └─► agentstream.Decoder
                         │
                         └─► usage.Reader ──► 会话用量

HTTP
  ├─ GET /internal/conversations
  └─ GET /internal/conversations/{sessionID}
```

本模块不读取 SQL。会话事件来自框架 Session Service；用量来自 `usage.Reader`。
