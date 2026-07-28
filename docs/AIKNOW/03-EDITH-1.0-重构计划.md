# EDITH 1.0 重构计划

> 定位：一个多用户 Agent 服务端 Runtime。以 `userID` 为用户资源隔离锚点，以 RunOptions 为每次运行的装配入口。

---

## 1. 1.0 必须交付什么

Go 后端接收 BFF 的内部请求：

```text
user_id + session_id + message
  → 读取用户配置
  → 取得 E2B sandbox
  → 组装 Prompt、MCP、Sandbox 工具和 RunOptions
  → shared Runner.Run
  → SSE Event 流返回 BFF
```

Web、Telegram、飞书、Slack 等都不是 1.0 Runtime 的职责。它们以后只需经 BFF / 适配层把同样的三元输入送入 Runtime。

## 2. 资源隔离与生命周期

| 资源 | 隔离键 | 生命周期 / 存放 |
|---|---|---|
| Session | `<edith, userID, sessionID>` | 单会话；框架 SessionService |
| Memory | `<edith, userID>` | 跨会话；框架 MemoryService |
| 模型配置 | `userID` | 用户私有；DB 配置，key 仅进入本次 Header |
| MCP 配置 | `userID` | 用户私有；每 Run 创建 HTTP ToolSet、Run 完关闭 |
| Skills | `userID` | 用户私有；Markdown + scripts，放 E2B Volume |
| Sandbox | `<userID, sessionID>` | 会话一沙箱；E2B SDK 管 pause/reconnect |

不变式：任何“用户级 X”读取、注入与扩展都必须以 `userID` 为起点；任何“会话级 X”再加 `sessionID`。

## 3. 明确的系统边界

| 层 | 负责 | 不负责 |
|---|---|---|
| Next.js BFF | 登录态、Clerk/Session、可信 userID 映射、对外 HTTP | Agent、工具、MCP、E2B |
| Go Runtime | Agent 运行、用户配置、Skills、MCP、Sandbox、SSE | Clerk、JWT、OAuth、前端 UI |
| tRPC-Agent-Go | Runner、LLM、工具循环、Session/Memory、Event、RunOptions | E2B、EDITH Skills、文件产物策略 |
| E2B SDK | Sandbox、Volume、pause/reconnect、文件和命令操作 | EDITH 的用户/会话路由与业务策略 |

Go Runtime 只可被 BFF 经 loopback 或私有网络调用。Go 信任 BFF 已经完成 userID 映射，不接受公网调用。

## 4. 代码组织：先直白，后扩展

第一版只需要清楚表达这些职责，避免为多云、多渠道、复杂 DDD 预建抽象：

```text
backend-v1/
  cmd/server/                 启动与依赖组装
  internal/http/              BFF 内部 HTTP/SSE handler
  internal/runtime/           单次 Agent Run 的服务编排与事件消费
  internal/runopts/           UserRuntime → []agent.RunOption
  internal/userconfig/        SQLite 中用户模型 / MCP / 性格配置读取
  internal/skills/            Skill 概览读取、E2B Volume 中 scripts 管理
  internal/mcp/               用户 MCP ToolSet 创建与关闭
  internal/sandbox/           按 <userID, sessionID> 调 E2B SDK；生成 sandbox tools
```

这不是强制的包名清单；原则是每个目录只表达一个业务职责。不要先定义 `StorageBackend`、`SandboxProvider`、渠道插件体系或多 Agent 编排。

## 5. 单次 Run 的资源所有权

运行服务负责整条生命周期，顺序固定：

```text
加载 UserRuntime
  → 获得或创建 <userID, sessionID> E2B sandbox
  → 创建本 Run 的 MCP ToolSet
  → 构建 AdditionalTools 和 RunOptions
  → Runner.Run
  → 消费 Event 流并持续采集 Usage
  → 收到 IsRunnerCompletion / channel 关闭
  → Close 本 Run MCP ToolSet，执行其他临时收尾
```

两个禁止事项：

- 不要在 `loadUserMCPTools` 内 `defer Close()`；会在 Run 开始前关闭。
- 不要只等 completion 才读取 Usage；在每个 Event 中记录最后一个非空 Usage。

## 6. Skills 与 Sandbox 的 EDITH 方案

Skills 是“Markdown 描述 + scripts 文件树”：

- 摘要用于本次 Run 的 `Instruction`，让模型理解可用能力。
- scripts 放在每个用户对应的 E2B Volume，跨 sandbox / 跨 session 复用。
- `userID ↔ volumeID + volumeToken` 的映射存 SQLite。
- sandbox 内 uploads、临时中间物和产物保留在它自己的 `/home/user/`；其存续由 E2B pause/reconnect 策略决定。

EDITH 1.0 允许深度绑定 E2B，不创建泛化的存储后端接口。

## 7. 实现顺序

1. 从 `forward/` 已验证骨架建立 shared Runner、零默认 Agent 和 Event/SSE 循环。
2. 接 SQLite：读取用户模型、API Key、性格、MCP、Skills 配置。
3. 实现 `UserRuntime` 与 `runopts.Build`，仅使用 7 个核心 RunOptions 字段。
4. 接用户 MCP：每 Run 创建 HTTP ToolSet，验证用户间工具与 key 隔离。
5. 接 E2B：按 `<userID, sessionID>` 获取 / 创建 sandbox，完成 pause/reconnect 与 Volume 挂载。
6. 提供 EDITH sandbox tools；Skill scripts 可在该用户 sandbox 中运行。
7. 接 BFF 内部 SSE 协议；再接 Web UI。其他 IM 渠道后置。

每一步都应能独立运行和验证，不等待后续模块才可测试。

## 8. 已验证、暂缓与不做

`forward/` 已验证：多模型切换、用户 Header API Key、Prompt 覆盖、`AdditionalTools`、远程 MCP、Event 流、Usage 采集与 ctx 取消传播。

暂缓：后台 Detached Run、集中取消平面、配额、角色权限审批、知识库、Graph / 多 Agent、复杂可观测。

不做：每用户 Agent、框架 CodeExecutor/Artifact/Skills、MCP STDIO 下载执行、泛化多云存储抽象。

## 9. 完成标准

当 BFF 传入两个不同用户和多个会话时，系统能证明：

- 各用户使用自己的模型 API Key、MCP 配置、Skill 概览和 Volume；
- 各会话只获得自己的 E2B Sandbox；
- 用户之间不会看到对方的 Session、Memory、工具、文件或沙箱；
- SSE 能正确流式转发文本、错误与完成信号；
- 断开请求会通过 ctx 取消整条运行链路并完成资源收尾。

