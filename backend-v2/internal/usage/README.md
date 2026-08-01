# usage

记录一次 Agent 运行的 Token，并按会话汇总。

```text
AgentRun
  ├─ Recorder.Start / Finish / Fail ──► usage_runs
  └─ Conversation ──► Reader.SessionSummary

Module
  ├─ Recorder  写入运行与 Token
  └─ Reader    读取状态与会话汇总
```

`store.go` 是私有 SQLite 细节；其他模块只使用 `Recorder` 或 `Reader`。
