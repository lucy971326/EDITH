# userconfig

用户配置模块：保存用户运行设置、供应商密钥、MCP 服务和渠道绑定。

```text
Module
├─ Settings   人格 / 默认模型 / 时区
├─ Providers  API Key（只在服务端读取）
├─ MCP        MCP 服务与私密 Header
├─ Bindings   渠道账号 → Clerk 用户
└─ HTTP       设置与 MCP 的 Web BFF 路由

外部模块 ──► Settings / Providers / MCP / Bindings
                    │
                    ▼
              私有 Store ──► SQLite
```

`HTTP.Register(mux)` 只定义本模块路由；是否注册由 `main` 决定。

`MCP.OpenTools` 只为一次 Agent Run 建立连接，并把关闭函数交还 AgentRun。
