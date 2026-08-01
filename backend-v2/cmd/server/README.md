# server

`main.go` 只做显式组装、路由注册和生命周期管理。

```text
打开 DB / Session
        │
        ▼
创建功能模块
        │
        ▼
创建 Tools / Agent / Runner
        │
        ▼
创建 AgentRun / Gateway / Adapter
        │
        ▼
注册路由 ──► 启动 Scheduler ──► 启动 HTTP
```

模块内部器官不在 main 创建；顶层模块之间的连接不藏进构造函数。
