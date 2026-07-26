# RunOptions 与服务端 Agent 设计

> 一个 Runner + RunOptions = 服务端 Agent 的全部秘诀。

---

## 一、框架为服务端 Agent 准备的设计

`tRPC-Agent-Go` 的 RunOptions 不是"框架 API 凑数"，而是为**多用户服务端场景量身设计**的字段集合。按"为什么服务端需要它"分类：

### 1. 多用户隔离维度

| 字段 | 解决的服务端问题 |
|---|---|
| `AppName` | 一个 Runner 服务多个项目/租户（session 写到不同 appName 下） |
| `EventFilterKey` | 隔离不同用户群的事件过滤范围 |
| `RuntimeState` | 把"房间 ID、租户上下文"等动态参数注入 Graph 运行时 |

### 2. 请求生命周期

| 字段 | 解决的服务端问题 |
|---|---|
| `RequestID` | 跨 goroutine 取消/查状态、追踪定位 |
| `DetachedCancel` | 父 ctx 取消不该影响运行中的 long-running run |
| `MaxRunDuration` | 每个用户单独的硬截止时长（VIP 用户更宽松） |
| `Resume` | 长会话中断后接着跑 |

### 3. 模型分流（多用户最关键）

| 字段 | 解决的服务端问题 |
|---|---|
| `Model` | 直接注入模型实例 |
| `ModelName` | 按名字查已注册的模型 |
| `ModelSelector` | 每次 LLM 调用前动态选模型 |
| `ModelContextWindow` | 覆盖上下文窗口 |
| `ModelRequestExtraFields` | provider 特定字段（DeepSeek thinking vs OpenAI reasoning） |
| `ModelRequestHeaders` | **多用户多租户关键**——每个用户自己的 API key 和鉴权头 |
| `Stream` | 单次请求切换流式/非流式 |

### 4. Prompt 动态化

| 字段 | 解决的服务端问题 |
|---|---|
| `Instruction` | 覆盖 Agent 的 instruction（每个用户 system prompt 不同） |
| `GlobalInstruction` | 覆盖全局 system prompt 前缀 |
| `InjectedContextMessages` | 历史前注入背景消息（不持久化） |
| `LateContextMessages` | 贴近最新用户回合注入动态规则（不持久化，cache 友好） |
| `UserMessageRewriter` | 改写用户消息，1→N 展开 |

### 5. 工具隔离（多用户权限）

| 字段 | 解决的服务端问题 |
|---|---|
| `ToolFilter` | 不同角色看到的工具不同（管理员能用 delete 工具） |
| `AdditionalTools` | 临时追加工具（专属工具） |
| `ExternalTools` | 暴露给模型但框架不执行（前端执行） |
| `ToolExecutionFilter` | 哪些工具调用实际执行 |
| `ToolPermissionPolicy` | 工具调用前审批（危险操作） |

### 6. 执行环境隔离

| 字段 | 解决的服务端问题 |
|---|---|
| `CodeExecutor` | 每用户单独沙箱 |
| `WorkspaceExecGuidance` | 覆盖 skill 的 workspace 提示 |
| `AvailableSkillsRenderer` | 自定义 skills 列表渲染 |

### 7. 数据输入

| 字段 | 解决的服务端问题 |
|---|---|
| `Messages` | 上游服务自己维护对话历史，不依赖 server 端持久化 |
| `KnowledgeFilter` / `KnowledgeConditionedFilter` | 不同用户看到不同知识库 |

### 8. 可观测与流控

| 字段 | 解决的服务端问题 |
|---|---|
| `StreamModeEnabled` / `StreamModes` | 不同客户端拿不同事件流 |
| `PersistInterruptedAssistant` | 中断时持久化已生成文本（"从断点继续"场景） |
| `GraphEmitFinalModelResponses` / `GraphTerminalMessagesOnly` | Graph 节点的事件暴露控制 |
| `DisableTracing` | 关闭 OpenTelemetry 减少开销 |
| `DisableResponseUsageTracking` | 关闭 usage 统计 |
| `ExecutionTraceEnabled` | 录制执行图 |
| `MaxLLMCalls` / `MaxToolIterations` | Invocation 级别的安全限制 |
| `ToolsArguments/Text JSON Repair` | 自动修复模型输出异常 |
| `Plugins` | Run 级插件（横切规则） |

> **RunOptions 的本质**：把"框架默认配置"按需**临时覆盖**给某次 Run。

---

## 二、五层职责分担

`Runner.Run(ctx, userID, sessionID, message, opts...)` 五个层次的分工：

### 第一层：Model 默认配置

**省下**：连接复用、客户端初始化、限流配置、Provider 切换、tokenizer 缓存。

**每次 Run 不关心**：连谁、怎么连、什么 API key。

**省心**：只关心"用哪个模型"这个名字。

### 第二层：LLMAgent 默认配置

**省下**：能力装载、默认 prompt、默认工具集、生成参数、记忆策略、重试策略。

**每次 Run 不关心**：工具的注册顺序、默认温度、默认 token 上限、默认 system prompt 模板。

**省心**：所有"每个用户都一样的东西"。

### 第三层：Runner 默认配置

**省下**：服务注册（Session/Memory/Artifact）、资源编排（Sandbox/Tool）、生命周期管理。

**每次 Run 不关心**：用 SQLite 还是 Redis、连接池管理、Close 归属。

**省心**：底层依赖换不换不用动业务代码。

### 第四层：Run 的前三参数（userID, sessionID, message）

**省下的不是配置，是义务**——必须每次 Run 都给：

- `userID` 由登录态决定
- `sessionID` 由客户端生成
- `message` 由用户发来

**省心**：Runner 不需要"猜"这次是谁、对谁、说什么——用户给得清清楚楚。

### 第五层：RunOptions 承担服务端 Agent 的核心职责

服务端 Agent 的"灵魂"——所有"按用户每次 Run 重新算"的差异化。

具体来说，RunOptions **必须承担**这些才算"服务端 Agent"：

| 必须承担 | 实现字段 |
|---|---|
| 多用户隔离 | AppName / RuntimeState |
| 请求可追踪 | RequestID |
| 生命周期控制 | DetachedCancel / MaxRunDuration |
| 模型分流 | Model / ModelName / Headers / ExtraFields |
| Prompt 动态化 | Instruction / GlobalInstruction / LateContext |
| 工具动态裁剪 | ToolFilter / AdditionalTools / Permission |
| 执行环境隔离 | CodeExecutor |
| 可观测性开关 | DisableTracing / StreamModes |

**RunOptions 不承担**：
- Agent 的长期能力（LLMAgent 字段）
- 后端服务选型（Runner 字段）
- Provider 连接管理（Model 字段）

---

## 三、服务端 Agent 本质

**服务端 Agent 不是"Agent 加上 HTTP"——它是新工程范式**。

### 客户端 vs 服务端的认知反转

| 客户端思维 | 服务端思维 |
|---|---|
| Agent 是中心 | Runner 是中心 |
| 配置 Agent 一次 | 配置 Runner 一次 |
| `agent.run(msg)` | `runner.Run(userID, sessionID, msg, opts...)` |
| 单用户单线程 | N 用户 × N 并发 |
| 没有 userID | **userID 是必填项** |
| 没有 sessionID | **sessionID 是会话标识** |
| 配置是写死的 | **配置是按请求动态注入的** |

**关键反转**：客户端**配置**，服务端**装配**。

### 服务端 Agent 独有的五件事

任何客户端 Agent 库移植到服务端都要补这块：

1. **多用户隔离** — session key 设计、并发安全
2. **个性化装配** — RunOptions 各字段协调
3. **上下文管理** — 长会话压缩、记忆/对话分离
4. **资源编排** — 沙箱、Tool 调用、MCP 连接的生命周期
5. **可观测** — Tracing、审计、token 计费

**真正难度**：这些事**同时发生**且**互相影响**——一个用户的工具调用完可能污染另一个用户的沙箱，模型切换可能让会话历史格式不对，记忆预加载可能把 A 的偏好泄漏给 B。

### 上下文也不同

客户端 Agent 上下文是**单会话**（进程在就有）。服务端是**多用户跨进程序列化**——Session Service 持久化、`<AppName, UserID, SessionID>` 三元组隔离、进程重启后能加载。

---

## 四、关键结论

> **服务端 Agent 意味着：每次请求都是定制化的——配置 + 上下文。**

- **配置定制化**（RunOptions 承担）：模型、prompt、工具、沙箱、权限，所有维度都能按用户每次 Run 不同
- **上下文定制化**（Session/Memory 承担）：每个用户自己的对话历史、自己的记忆、自己的 skill overview

**这两条加起来**：同一种代码骨架（一个 Runner + 一个 LLMAgent），服务 N 个用户，每个用户的每次 Run 都是独一份的 Agent 实例化。

### 一句话

> **默认配置（Model / LLMAgent / Runner）是"骨架"——一次性弄对就稳定。**
>
> **前三个参数是"谁、对谁、说什么"——必须每次 Run 都给。**
>
> **RunOptions 是"骨架盖好了，这次要盖成什么样"——服务端 Agent 的灵魂就在这里。**

---

## 五、客户端/服务端代码味道自检表

如果你看到这些代码味道，就知道是"客户端思维在写服务端"：

| 客户端味道 | 服务端应该长啥样 |
|---|---|
| `agent = create_agent(...)` 在请求处理里 | Agent 在启动时建好，**复用**到所有请求 |
| `prompt = "你是 X"` 字面量 | `prompt = template(userCtx)` 函数 |
| `agent.run(msg)` 不带 userID | `r.Run(ctx, userID, sessionID, msg, opts...)` |
| 用全局 `var currentUser` | 通过 **Invocation** / **RunOptions** 传 |
| "模型" 是字符串字面量 | **ModelSelector / ModelName 按用户路由** |
| 工具全量给 | **ToolFilter / PermissionPolicy 按用户裁剪** |
