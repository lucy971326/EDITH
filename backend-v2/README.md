# EDITH backend-v2

backend-v2 以显式依赖和纵向功能模块组织服务端代码。

```text
渠道 Adapter
      │
      ▼
   Gateway            身份与会话转换
      │
      ▼
   AgentRun           聚合运行配置
      │
      ▼
ManagedRunner.Run     框架执行入口
      │
      ▼
 agentstream          输出 EDITH 中性事件
```

`cmd/server/main.go` 创建顶层模块、连接依赖、注册路由并启动主动组件。每个功能模块拥有自己的数据、业务能力、HTTP 契约和路由。
