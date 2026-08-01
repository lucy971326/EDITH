# webadapter

WebAdapter 是 Web 渠道的协议适配层，只理解 HTTP、JSON 和 SSE。
它不加载模型、不读用户配置、不执行 Agent。

## 一张图看懂主链路

```text
Web BFF
  │
  ├─ POST /internal/gateway/messages:stream
  │    JSON MessageRequest
  │
  ├─ GET  /internal/gateway/runs/{requestID}
  └─ POST /internal/gateway/runs/{requestID}/cancel
       │
       ▼
Adapter
├─ 解析 HTTP 输入
├─ 调用 Gateway
├─ 把 gateway.Event 写成 SSE
└─ 把 gateway.Error 翻译成 HTTP 状态码
       │
       ▼
gateway.Service
└─ IncomingMessage → agentrun.Request → ManagedRunner
       │
       ▼
中性 gateway.Event channel
       │
       ▼
浏览器 SSE
```

## 对外结构

```text
Adapter
└─ gateway *gateway.Service
   └─ WebAdapter 唯一外部依赖
```

```text
New(agentGateway)
└─ 校验 Gateway，返回 Adapter

Register(mux)
├─ POST /internal/gateway/messages:stream → StreamMessage
├─ GET  /internal/gateway/runs/{requestID} → RunStatus
└─ POST /internal/gateway/runs/{requestID}/cancel → CancelRun
```

main 只创建 Adapter 并注册路由：

```text
agentGateway := gateway.New(...)
web := webadapter.New(agentGateway)
web.Register(mux)
```

Adapter 不需要额外的 Module；它只有一个对外能力：Web 渠道适配。

## MessageRequest：Web 消息输入契约

```text
MessageRequest
├─ RequestID         本次运行 ID
├─ UserID            BFF 注入的 Clerk 用户 ID
├─ SessionID         会话 ID
├─ Message           用户文本
├─ ImageIDs          已确认图片 ID
├─ ModelID           聊天页临时选择的模型
└─ ReasoningOptionID 推理选项
```

UserID 必须由 Web BFF 从 Clerk 身份注入，浏览器不能自行决定用户身份。

StreamMessage 的翻译：

```text
MessageRequest
      ↓
gateway.IncomingMessage
├─ Channel           web
├─ ExternalUserID    UserID
├─ SessionKey        SessionID
├─ RequestID
├─ Message
├─ ImageIDs
├─ ModelID
└─ ReasoningOptionID
      ↓
gateway.Run
```

## StreamMessage：流式消息入口

```text
POST /internal/gateway/messages:stream
├─ 限制请求体 1 MiB
├─ 读取 MessageRequest
├─ 调 Gateway.Run
├─ Gateway 错误 → HTTP JSON 错误
├─ 成功 → 设置 text/event-stream
└─ range stream.Events
   ├─ writeSSE
   ├─ data: {JSON Event}\n\n
   └─ Flush
```

SSE 输出只有中性事件，不在 Adapter 中转换为 Timeline：

```text
gateway.Event
└─ JSON
   └─ data: {...}\n\n
      └─ 浏览器前端翻译为 Timeline
```

## 浏览器断线时的处理

```text
SSE 写入失败
├─ clientConnected = false
├─ 不再向 ResponseWriter 写入
├─ 继续消费 stream.Events
└─ 直到 channel 关闭
   └─ AgentRun 完成 MCP、Usage、lane、取消标记收尾
```

所以 Adapter 的 HTTP 连接断开，不会让 AgentRun 失去收尾机会。

## RunStatus：活跃任务查询

```text
GET /internal/gateway/runs/{requestID}?userId=...
├─ 读取 query userId
├─ 读取 path requestID
├─ 调 Gateway.Status
└─ 返回 JSON gateway.Status
```

Gateway 再向 ManagedRunner 查询活跃任务并校验用户归属。
SQLite 不是当前运行状态的真相源。

## CancelRun：任务取消入口

```text
POST /internal/gateway/runs/{requestID}/cancel?userId=...
├─ 读取 query userId
├─ 读取 path requestID
├─ 调 Gateway.Cancel
└─ 成功返回 204 No Content
```

Gateway 校验任务归属后调用 ManagedRunner.Cancel。
取消是否成功以后端 ManagedRunner 为准。

## 错误转换

```text
gateway.Error.Type       HTTP 状态
invalid_request          400 Bad Request
session_busy             409 Conflict
request_conflict         409 Conflict
identity_not_bound       403 Forbidden
not_found                404 Not Found
其他                     500 Internal Server Error
```

## writeSSE

```text
writeSSE(writer, gateway.Event)
├─ json.Marshal(event)
├─ 写入 data: JSON + 空行
├─ http.Flusher.Flush
└─ 返回写入错误
```

它只负责 SSE 线格式，不理解事件业务含义。

## 一句话记忆

```text
MessageRequest       = Web JSON 输入
IncomingMessage      = Gateway 中性输入
gateway.Event        = Gateway 中性输出
writeSSE             = 中性事件 → SSE 线路
StreamMessage        = 启动并消费完整事件流
RunStatus / CancelRun= 转发后端状态控制
Adapter              = 只做协议翻译，不做 Agent 业务
```
