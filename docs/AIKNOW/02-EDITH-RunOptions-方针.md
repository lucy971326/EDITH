# EDITH RunOptions 方针：服务端 Agent 的装配方式

> 核心口号：**用户差异都在 RunOptions 里表达。**
> 
> 本文是 EDITH 服务端设计的第一原则。细节验证记录见 `docs/learn/RunOptions/`。

---

## 1. 认知反转

客户端 Agent 的惯性是“为一个人配置一个 Agent”；服务端 Agent 应是“预先建好骨架，按请求装配一次运行”。

```text
错误：HTTP handler → new Agent(用户 key、用户 prompt、用户工具) → run
正确：HTTP handler → 读取用户运行快照 → BuildRunOptions → shared Runner.Run
```

EDITH 只有一个长期 Runner，所有 Web / SSE / Telegram / 未来渠道共用。渠道的职责只是传入可信 `userID`、`sessionID`、消息和渠道名。

## 2. Run 的固定形状

```go
opts, cleanup, err := runopts.Build(ctx, channel, userRuntime)
if err != nil { /* 返回错误 */ }
defer cleanup() // 注意：必须在事件流被完整消费后才执行

events, err := edithRunner.Run(ctx, userID, sessionID, message, opts...)
```

其中 `userRuntime` 是 EDITH 从配置、Skill、MCP、Sandbox 服务得到的**一次运行快照**。`runopts.Build` 不应查询数据库、不应写业务状态；它只负责把快照翻译成框架 RunOptions。

## 3. 当前 7 个核心字段

| 字段 | 每次 Run 的含义 |
|---|---|
| `WithRequestID` | UUID 链路追踪；未来集中取消的键 |
| `WithStream` | 渠道输出策略：Web/SSE 为 true，普通 IM 通常为 false |
| `WithModelName` | 用户本次选择的、已注册的模型名称 |
| `WithModelRequestHeaders` | 用户自己的 API Key，例如 Authorization Header |
| `WithGlobalInstruction` | L1 开发者身份 + L2 用户 Agent 性格 |
| `WithInstruction` | L3 系统 Skill 概览 + L4 用户私有 Skill 概览 |
| `WithAdditionalTools` | 用户本次可用的 MCP / Sandbox 工具 |

这是 EDITH 1.0 的完整核心，不因为框架还有更多字段就提前启用更多字段。

## 4. 三类输入，职责绝不混淆

```text
Runner.Run 的前三参数：谁、在哪段对话、这次说什么
  userID, sessionID, message

RunOptions：这一次以什么个性、模型和能力运行
  模型、key、prompt、动态工具、流式策略

长期骨架：所有用户都一样且不会按请求变化的能力
  Runner、Agent、共享 Model 实例、Session/Memory 服务、系统工具
```

`userID` 由 BFF 的登录态映射而来，Go Runtime 不解析 Clerk/JWT/OAuth。Go 仅接受来自内部 BFF 的请求，不能对公网暴露。

## 5. 模型与 API Key：共享连接，隔离凭据

Model 实例按“供应商 / base URL / 模型能力”注册在进程中，可以被所有用户复用；它**不携带用户 API Key**。

```text
共享 Model: deepseek-v4 / minimax-m3 / ...
Run A: ModelName=deepseek-v4 + Header=Alice 的 key
Run B: ModelName=minimax-m3  + Header=Bob 的 key
```

`ModelName` 只能选择服务端已经注册和允许使用的模型，不能让客户端任意传供应商 URL 或模型配置。用户配置是服务端读取、校验后再装配，不是客户端直传。

## 6. Prompt 的六层

模型看到的上下文按此概念组织：

```text
L1 开发者身份（稳定） + L2 用户性格（用户配置）  → GlobalInstruction
L3 系统 Skills 概览 + L4 用户 Skills 概览           → Instruction
L6 Session 历史                                     → 框架自动加载
L5 当前用户消息                                     → Runner.Run(message)
```

不要在长期 `LLMAgent` 放默认 Prompt 来掩盖漏装配错误。L1 至 L4 都必须显式由每次 Run 传入；漏传应及早暴露。

EDITH 不使用框架 `WithSkills(repo)`、`skill_load` 或 `InjectedContextMessages`。Skills 是 EDITH 自己的 Markdown + scripts 系统；当前阶段向模型暴露的是经过整理的概览，不是整个脚本内容。

## 7. 动态工具：每用户、每 Run

```text
默认系统工具（长期 Agent）
  + 用户自己的 MCP ToolSet 转出的 tools
  + 本 <userID, sessionID> 的 sandbox tools
  = WithAdditionalTools(...)
```

MCP 只能支持远程 HTTP transport（SSE 或 streamable HTTP），不能允许平台替用户下载并执行 STDIO server。

每用户每 Run 重建 MCP ToolSet 是 1.0 的隔离优先策略。资源收尾必须由执行 Run 的服务层负责：创建 ToolSet → 消费完整 Event 流 → Close ToolSet。`AdditionalTools` 本身不拥有 Close 生命周期。

## 8. ctx、取消与资源收尾

在线 SSE 模式直接把请求 `ctx` 传给 Runner：浏览器断连，整条模型/工具/MCP 链路取消。

每次仍生成 `RequestID`，为以后提供：

- 用户点击停止；
- 配额耗尽；
- 运维中止；
- 状态追踪。

任何情况下都要 drain / 消费 Event channel，并让 MCP ToolSet 和 EDITH 临时资源走同一条收尾路径。不要为了后台任务提前采用 `DetachedCancel`；只有真实后台运行需求出现时再启用。

## 9. 现在明确不用的东西

| 不用 | 原因 |
|---|---|
| 每用户 Agent / Runner | 违反共享骨架、请求装配原则 |
| `WithAppName` | EDITH 的 appName 固定 |
| `RunOptions.Messages` | SessionService 自动维护历史 |
| `ToolFilter` / 权限审批 | 1.0 尚无角色与审批业务；用户工具由装配决定 |
| `CodeExecutor` / Artifact / 框架 Skills | 是本地 Agent 语义，不是 EDITH 平台边界 |
| GraphAgent / 多 Agent 编排 | 1.0 不需要复杂流程 |

未来需求出现时再考虑 `MaxRunDuration`、`ManagedRunner.Cancel`、Tool Permission、Knowledge Filter、Tracing 等字段。

## 10. 新代码的自检

如果新增功能是“某用户本次运行才不同”，默认先问：能否把选择结果放进 `UserRuntime`，再由 `BuildRunOptions` 翻译？

如果答案是否定的，先说明它为什么是长期共享骨架或 EDITH 的独立资源生命周期；不要习惯性把配置写进 Agent 字段或 package 全局变量。

