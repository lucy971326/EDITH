# agentstream

`agentstream` 保存共享的事件解释规则；每个 `Decoder` 只保存一次回复的文本块状态。

```text
框架 event.Event
        │
        ▼
Decoder.DecodeFrameworkEvent
   ├─ Events ───────► reasoning.delta
   │                 message.delta
   │                 tool.started / finished
   │
   └─ 运行侧信号 ───► Completed / ErrorMessage / Usage
```

`Events` 是对外的 EDITH 中性事件；运行侧信号只交给 AgentRun 做收尾。

Tool 会结束当前文本块，因此 Tool 后面的文字一定获得新的 BlockID。实时流和历史投影复用同一解释规则，但各自创建 Decoder。
