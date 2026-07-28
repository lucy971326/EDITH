# 模型收到的上下文构造


```txt
模型实际收到的 req.Messages（自上而下）:
═══════════════════════════════════════════════════════════════════

[SY]  System Message（一个块，多处理器合并）
      ├─ GlobalInstruction            ← RunOptions.GlobalInstruction
      ├─ Instruction                  ← RunOptions.Instruction
      ├─ Identity / Skills / Workspace / PostTool / Time / Memory / Recall
      │   (不同处理器都塞进这条 system 块)
      └─ JSON Schema 模板             (Instruction 尾部)

[FS]  Few-shot examples              (Agent 长期配置)
                                       紧跟 leading system block 之后

[IN]  InjectedContextMessages        ← RunOptions.InjectedContextMessages
      ├─ 位置: system 之后、few-shot 之后、history 之前
      └─ 不持久化（设计保证）

[HS]  Session History
      ├─ User-mode 注入（Summary/Memory/Recall）prepend 到第一条 user msg
      └─ ToolTranscriptMode / 上下文压缩

[UM]  Invocation.Message
      ├─ 来自 UserMessageRewriter 改写后的最后一条
      └─ 持久化到 session_events

[LC]  LateContextMessages            ← RunOptions.LateContextMessages
      ├─ 位置: "最后一个 user message 之前"（不是末尾）
      └─ 不持久化（设计保证）

═══════════════════════════════════════════════════════════════════

```