# cronadapter

CronAdapter 与 WebAdapter 平级，只负责渠道转换。

```text
cronjob.Scheduler
      │ Job
      ▼
CronAdapter.RunJob
      │ IncomingMessage{Channel: cron}
      ▼
Gateway ──► AgentRun
      │
      ▼
消费中性事件直到关闭
```

Scheduler 管理何时执行，CronAdapter 只管理如何进入 Gateway。
