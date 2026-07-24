# 07 - AG-UI 前端协议

> AG-UI = Agent ↔ 前端的协议适配层。基于 SSE，把 `event.Event` 翻译成前端事件流。

---

## 1. 做什么的

把框架内部的 Agent 执行事件通过 **SSE（Server-Sent Events）** 推送到前端，符合 [AG-UI 开放协议](https://docs.ag-ui.com/)。

```
前端 (CopilotKit / TDesign Chat / 自研 Web)
   ↓ SSE (AG-UI 协议事件流)
Server (server/agui/agui.go)       ← HTTP 入口，管理路由
   ↓
Service (service/sse/)             ← 传输协议抽象（默认 SSE，可换 WebSocket）
   ↓
AG-UI Runner (runner/runner.go)    ← 协议适配：AG-UI 请求 → Runner.Run + 事件翻译
   ↓
Framework Runner / Translator      ← 底层 Agent 执行 + 事件 → AG-UI 事件
```

---

## 2. 三条路由

| 路由 | 默认路径 | 职责 | 详细文档 |
|---|---|---|---|
| **Chat** | `/` | 接收对话请求，SSE 流式返回 AG-UI 事件 | `agui/chat.md` |
| **History** | `/history` | 从 Session Track 恢复历史消息（MESSAGES_SNAPSHOT） | `agui/history.md` |
| **Cancel** | `/cancel` | 根据 SessionKey 取消正在运行的请求 | `agui/cancel.md` |

可通过 `agui.WithPath("/agui")` 修改路由前缀（详见 `agui/index.md#路由前缀`）。

---

## 3. 快速上手

```go
import (
    "trpc.group/trpc-go/trpc-agent-go/runner"
    "trpc.group/trpc-go/trpc-agent-go/server/agui"
)

r := runner.NewRunner("my-app", agent)
defer r.Close()

server, _ := agui.New(r,
    agui.WithPath("/agui"),  // 默认 "/"
)
http.Handle("/agui", server.Handler())
http.ListenAndServe(":8080", nil)
```

完整文档：`agui/index.md#快速上手`

---

## 4. Chat 路由（实时对话）

### 4.1 请求体 RunAgentInput

```go
type RunAgentInput struct {
    ThreadID   string                 // 会话 ID
    RunID      string                 // 唯一 run 标识
    ParentID   string                 // 多 Agent 嵌套
    Messages   []Message              // 消息列表
    Tools      []Tool                 // 前端工具
    Context    []Context              // 上下文
    ForwardedProps json.RawMessage
    State      any
}
```

完整字段：`agui/chat.md#请求体-runagentinput`

### 4.2 Resolver 链

AG-UI Runner 通过 Resolver 链把 `RunAgentInput` 转换成 `Runner.Run` 调用：

```
RunAgentInputHook → UserIDResolver → RunOptionResolver → AppNameResolver → StateResolver
```

各 Resolver 详解：`agui/chat.md` 中搜索 "自定义 XxxResolver"

### 4.3 事件翻译映射

| 框架事件 | AG-UI 事件 |
|---|---|
| ChatCompletion（流式/完整） | `TEXT_MESSAGE_START → CONTENT → END` |
| ReasoningContent | `REASONING_START → MESSAGE_CONTENT → END → REASONING_END` |
| ToolCall | `TOOL_CALL_START → ARGS → END` |
| ToolResult | `TOOL_CALL_RESULT` |
| runner.completion | `RUN_STARTED / RUN_FINISHED / RUN_ERROR` |
| graph.node.lifecycle | `ACTIVITY_DELTA(graph.node.lifecycle)` |
| graph.node.interrupt | `ACTIVITY_DELTA(graph.node.interrupt)` |

### 4.4 Translator 状态机

每次 Run 创建一个 Translator，维护消息流 open/close 状态。`PostRunFinalizationEvents` 在结束时补齐未关闭的 TEXT_MESSAGE_END / REASONING_END / TOOL_CALL_END。

**串行 vs 并发**：
- 默认：同一时刻只开一个文本流
- `WithConcurrentMessageStreamsEnabled`：多 Agent 场景不同 messageId 可同时流式

### 4.5 翻译回调

```go
translatorCallbacks := translator.NewCallbacks()

translatorCallbacks.RegisterBeforeTranslate(func(ctx, ev *event.Event) (*event.Event, error) {
    // 拦截/替换框架事件
    return ev, nil
})

translatorCallbacks.RegisterAfterTranslate(func(ctx, ev aguievents.Event) (aguievents.Event, error) {
    // 拦截/替换 AG-UI 事件
    return ev, nil
})

aguiRunner := aguirunner.New(runner,
    aguirunner.WithTranslateCallbacks(translatorCallbacks),
)
```

**短路机制**：第一个返回非 nil 的生效。详见 `agui/chat.md#事件翻译回调`。

---

## 5. History 路由（消息快照）

### 5.1 核心概念

从 Session Track 恢复历史消息，发 `MESSAGES_SNAPSHOT` 事件给前端。

### 5.2 开启消息快照

```go
aguiRunner := aguirunner.New(runner,
    aguirunner.WithMessageSnapshotEnabled(true),
)
```

### 5.3 事件聚合（Aggregator）

合并连续增量事件后写 Track，默认 **1s 刷新**，减少存储 IO。

### 5.4 Track 持久化

AG-UI 事件写入 `SessionService.Track`，供 History 重放。**不污染主事件流**。

### 5.5 已知限制（EDITH 重点）

⚠️ **History 路由的消息快照只读 track events**，无法从数据库读取真实的历史对话流。

**解决方案（EDITH 当前采用）**：自定义 History 路由（webhook 方式），从自己的数据库表（如 EDITH 的 messages 表）查询历史消息，转成 AG-UI `MESSAGES_SNAPSHOT` 事件。

详见 `agui/history.md`。

---

## 6. Cancel 路由（取消运行）

### 6.1 取消请求

前端发请求到 `/cancel`，AG-UI Runner 找到对应 SessionKey 的运行并停止。

### 6.2 多实例分布式取消

通过共享 SessionService 写标记，持有运行的实例轮询检查。

详见 `agui/cancel.md`。

---

## 7. 高级特性

| 特性 | 配置 / 说明 | 文档 |
|---|---|---|
| 思考内容（reasoning） | DeepSeek 等推理模型输出 | `agui/chat.md#思考内容` |
| 流式工具调用参数 | ToolCall 参数增量传输 | `agui/chat.md#流式工具调用参数` |
| 流式工具执行结果 | 工具结果流式返回 | `agui/chat.md#流式工具执行结果` |
| 事件来源元数据 | Author / Tag 透传 | `agui/chat.md#事件来源元数据` |
| 外部工具 | 前端注册工具给 Agent 用 | `agui/chat.md#外部工具` |
| GraphAgent 节点活动事件 | graph.node.* 事件 | `agui/chat.md#graphagent-节点活动事件` |
| 并发消息流 | 多 Agent 并行流式 | `agui/chat.md#并发消息流` |
| 自定义传输协议 | 换 SSE 为 WebSocket | `agui/chat.md#自定义传输协议` |

---

## 8. SSE 心跳保活

长连接需要心跳防止中间设备超时。详见 `agui/chat.md#sse-心跳保活`。

---

## 9. 并发控制

同一 SessionKey 同时只允许一个实时对话，重复返回 **409**。

```go
aguiRunner := aguirunner.New(runner,
    aguirunner.WithSessionConcurrency(1),  // 每 SessionKey 最多 1 个
)
```

---

## 10. 前端选型

| 框架 | 文档 / 示例 |
|---|---|
| **TDesign Chat**（React + Vite） | `examples/agui/client/tdesign-chat` |
| **CopilotKit** | `examples/agui/client/copilotkit` |

完整接入步骤：`agui/index.md` + 各前端框架官方文档。

---

## 11. 踩坑提醒

| 坑 | 解法 |
|---|---|
| History 路由读不到真实历史 | 自定义 History webhook，从自己 DB 读 |
| 多 Agent 串行太慢 | 开启 `WithConcurrentMessageStreamsEnabled` |
| Translator 忘记关 message stream | 用 `PostRunFinalizationEvents` 兜底 |
| SSE 长连接被中间设备断开 | 配置心跳 + 前端自动重连 |
| 并发跑同一 SessionKey | 默认 409，多实例用分布式取消 |
| 前端 CopilotKit 用 v1 API | 用 v2（GraphQL 已废弃，改 AG-UI） |
| Event 流里有 chat.completion + tool.response 顺序错 | 检查 Translator 状态机 |

---

## 12. 去哪查

- **总览**：`docs/trpc-agent-go/docs/mkdocs/zh/agui/index.md`
- **Chat 路由**：`docs/trpc-agent-go/docs/mkdocs/zh/agui/chat.md`
- **History 路由**：`docs/trpc-agent-go/docs/mkdocs/zh/agui/history.md`
- **Cancel 路由**：`docs/trpc-agent-go/docs/mkdocs/zh/agui/cancel.md`
- **A2A 协议**：`docs/trpc-agent-go/docs/mkdocs/zh/a2a.md`（不同：Agent ↔ Agent 协议）
- **A2UI**：`docs/trpc-agent-go/docs/mkdocs/zh/a2ui.md`（AG-UI 之上的 UI 生成协议）
