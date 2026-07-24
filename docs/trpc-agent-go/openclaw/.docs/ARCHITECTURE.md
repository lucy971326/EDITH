# OpenClaw 架构速查 —— Channel / Gateway / Runner 三层关系

> 这份文档用来回顾"为什么 OpenClaw 是这样组织的"。
> 阅读时间 5 分钟；遇到具体问题时回来查。

---

## 目录

- [§ 1 一句话本质](#-1-一句话本质)
- [§ 2 三层架构图](#-2-三层架构图)
- [§ 3 每个角色干一件事](#-3-每个角色干一件事)
- [§ 4 关键数据结构](#-4-关键数据结构)
  - [4.1 ContentPart —— 多模态的"信封"](#41-contentpart--多模态的信封)
  - [4.2 MessageRequest / MessageResponse —— 协议无关的请求/响应](#42-messagerequest--messageresponse--协议无关的请求响应)
  - [4.3 StreamEvent —— 流式事件](#43-streamevent--流式事件)
  - [4.4 OutboundMessage / OutboundFile —— 出站载荷](#44-outboundmessage--outboundfile--出站载荷)
- [§ 5 接口三件套](#-5-接口三件套)
- [§ 6 启动时如何装配](#-6-启动时如何装配)
- [§ 7 消息接收全链路](#-7-消息接收全链路)
- [§ 8 文件接收全链路](#-8-文件接收全链路)
- [§ 9 文件发送全链路（重点）](#-9-文件发送全链路重点)
- [§ 10 Gateway HTTP 接口设计](#-10-gateway-http-接口设计)
- [§ 11 三条铁律（最容易出错的边界）](#-11-三条铁律最容易出错的边界)

---

## § 1 一句话本质

```
Channel 负责"主动监听"外部平台（推模式）
Gateway 负责"接收请求 + 标准化 + 交给 Runner"（HTTP 入口）
Runner  负责"真正让 Agent 干活 + 流式返回"
```

**三个东西的边界**：

| 组件 | 只关心 | 不关心 |
|---|---|---|
| **Channel** | 怎么连平台、怎么发消息 | AI 怎么推理 |
| **Gateway** | 怎么转换请求、怎么路由 | 平台细节 |
| **Runner** | 怎么让 Agent 干活 | 谁发来的、用什么平台 |

---

## § 2 三层架构图

```
                          ┌──────────────────────────────────────────────────┐
                          │  Channel 层（IM 平台适配）                       │
                          │                                                  │
                          │  ┌─────────────────┐    ┌─────────────────────┐  │
                          │  │ Telegram        │    │ stdin               │  │
                          │  │  - getUpdates   │    │  - 读一行           │  │
                          │  │  - sendDocument │    │  - 写一行           │  │
                          │  │  - 配对/允许名单│    │                     │  │
                          │  └────────┬────────┘    └──────────┬──────────┘  │
                          │           │                        │              │
                          │           └────────┬───────────────┘              │
                          │                    │ gw.SendMessage()           │
                          └────────────────────┼─────────────────────────────┘
                                               │
                                               ↓ gwclient 接口
                          ┌──────────────────────────────────────────────────┐
                          │  Gateway 层（HTTP 入口 + 标准化）                 │
                          │                                                  │
                          │  POST /v1/gateway/messages        ← 同步          │
                          │  POST /v1/gateway/messages:stream ← SSE          │
                          │  POST /v1/gateway/cancel                         │
                          │  GET  /v1/gateway/status?request_id=...           │
                          │  GET  /healthz                                   │
                          │                                                  │
                          │  ┌────────────────────────────────────────────┐  │
                          │  │ handleMessages / handleStream              │  │
                          │  │  - allowlist 检查                          │  │
                          │  │  - session_id 计算（dm/thread/topic）      │  │
                          │  │  - mention gating                          │  │
                          │  │  - 多模态 ContentPart 标准化               │  │
                          │  └────────────────┬───────────────────────────┘  │
                          └───────────────────┼──────────────────────────────┘
                                              │ runner.Run(ctx, userID, sessionID, msg)
                                              ↓
                          ┌──────────────────────────────────────────────────┐
                          │  Runner 层（trpc-agent-go 原生执行）              │
                          │                                                  │
                          │  Runner                                          │
                          │   ├─ Session Service  ← 对话历史                  │
                          │   ├─ Memory Service   ← 跨会话记忆              │
                          │   ├─ Agent (LLMAgent) ← 推理                     │
                          │   ├─ Tools / ToolSets ← 能力                     │
                          │   ├─ Skills           ← 技能                     │
                          │   └─ CodeExecutor     ← 沙箱                     │
                          │                                                  │
                          │  输出：<-chan *event.Event（流式）                │
                          └──────────────────────────────────────────────────┘
```

---

## § 3 每个角色干一件事

### Channel —— "一个具体的 IM 平台适配器"

**干一件事**：连到一个具体的 IM 平台，把平台消息转成 `MessageRequest` 发给 Gateway。

**最小接口**（`channel/channel.go`）：
```go
type Channel interface {
    ID() string                    // 这个 channel 叫什么
    Run(ctx context.Context) error // 启动后阻塞，监听平台事件
}
```

可选接口：`TextSender`（能发文本）/ `MessageSender`（能发文件）

**两种实现**：
- `plugins/stdin/stdin.go`：读 STDIN 一行一行当消息
- `internal/channel/telegram/channel.go`：调 Telegram `getUpdates` 长轮询

### Gateway —— "AI 系统的 HTTP 大门"

**干一件事**：把外面发来的 HTTP 请求，转成 `Runner.Run()` 调用，再把返回结果写回 HTTP 响应。

### Runner —— "AI 大脑"

trpc-agent-go 原生的 `Runner`。Channel 和 Gateway 都不直接调它——它们都通过 **gwclient（Gateway 客户端）** 间接调它。

---

## § 4 关键数据结构

### 4.1 ContentPart —— 多模态的"信封"

文件 / 图片 / 音频 / 视频 / 文本 / 位置 / 链接，**统一抽象成 4 种字段**：

```go
type ContentPart struct {
    Type     ContentPartType
    Text     *string
    Image    *ImagePart    // 图片
    Audio    *AudioPart    // 音频
    File     *FilePart     // 通用文件
    Location *LocationPart
    Link     *LinkPart
}

// 任意"文件"都能塞进 FilePart
type FilePart struct {
    Filename string
    Data     []byte    // 内联字节（HTTP webhook 直接传）
    FileID   string    // IM 平台 ID（Telegram 必须走这个，二阶段下载）
    Format   string    // MIME type
    URL      string    // 远程 URL（HTTP webhook 用这个）
}
```

**4 种传输方式**：
- `Data []byte` —— 字节直接塞请求体（HTTP webhook）
- `FileID` —— 平台内部 ID（Telegram 必须走这条路，因为 TG 不让你传字节）
- `URL` —— 远程链接（用户发了个 URL）
- Gateway 自己 `DownloadFileByID` 真的去下

### 4.2 MessageRequest / MessageResponse —— 协议无关的请求/响应

```go
type MessageRequest struct {
    Channel   string  // "telegram" | "stdin" | ...
    From      string  // 发送者 ID（Telegram user.id）
    To        string  // 接收者 ID
    Thread    string  // 会话线索（dm / chatID / topicID）
    MessageID string  // 平台消息 ID（用于 reply_to）
    Text      string  // 主文本

    ContentParts []ContentPart  // 多模态附件

    RequestSystemPrompt      string  // 临时 system prompt 注入
    RequestLateContextPrompt string  // 临时上下文注入

    UserID    string  // 用户 ID
    SessionID string  // 会话 ID
    RequestID string  // 本次请求 ID（用于 /cancel / /status）

    Extensions map[string]json.RawMessage
}
```

**关键设计**：`MessageRequest` 不含任何平台特定字段（Telegram 的 chat_id 类型、Slack 的 channel_id 命名等）。**Channel 负责把平台字段压成统一字段**。

### 4.3 StreamEvent —— 流式事件

```go
type StreamEvent struct {
    Type StreamEventType  // 见下表

    SessionID string
    RequestID string

    Delta      string  // 流式文本增量
    Reply      string  // 完整文本（run.completed 时）
    Stage      string  // "preparing" | "running_tool" | "summarizing" ...
    Summary    string  // 低频状态摘要
    ToolName   string
    ToolDetail string
    ToolCallID string
    ToolStatus string  // "running" | "ok" | "error"
    ElapsedMS  int64
    Usage      *Usage
    Ignored    bool
    Error      *APIError
}
```

**事件类型**（`StreamEventType`）：

| Type | 含义 |
|---|---|
| `run.started` | Run 开始 |
| `run.ignored` | 请求被忽略（allowlist / mention gating） |
| `run.progress` | 低频状态（preparing / running_tool / summarizing） |
| `message.delta` | 流式文本增量 |
| `message.completed` | 单条消息完成 |
| `run.completed` | 整次 Run 完成 |
| `run.error` | 错误 |

**典型成功流**：
```
run.started
  ↓
零或多个 run.progress
  ↓
零或多个 message.delta
  ↓
message.completed
  ↓
run.completed
```

### 4.4 OutboundMessage / OutboundFile —— 出站载荷

```go
// channel/channel.go
type OutboundMessage struct {
    Text  string
    Files []OutboundFile
}

type OutboundFile struct {
    Path    string  // 本地路径
    Name    string  // 改名（可选）
    AsVoice bool    // 音频当语音条发
}
```

**关键**：`Path` 是**本地文件路径**。Agent 不知道有 Channel，只知道路径。

---

## § 5 接口三件套

```go
// channel/channel.go

// 1. 最基础：能监听平台
type Channel interface {
    ID() string
    Run(ctx context.Context) error
}

// 2. 可选：能发文本
type TextSender interface {
    Channel
    SendText(ctx context.Context, target string, text string) error
}

// 3. 可选：能发文本 + 文件
type MessageSender interface {
    Channel
    SendMessage(ctx context.Context, target string, msg OutboundMessage) error
}

// gatewayClient —— Channel 用来发消息给 Gateway 的接口
type gatewayClient interface {
    SendMessage(ctx context.Context, req gwclient.MessageRequest) (gwclient.MessageResponse, error)
    Cancel(ctx context.Context, requestID string) (bool, error)
}
```

**Channel 同时实现 3 个接口很常见**（Telegram Channel 实现了 `Channel` + `TextSender` + `MessageSender`），因为它**组合了多种能力**。

---

## § 6 启动时如何装配

```go
// 装配顺序：Runner → Gateway → gwclient → Channel → HTTP 监听
//
// 1. 先建 Runner（核心）
runner := runner.NewRunner(appName, agent, opts...)

// 2. 用 Runner 建 Gateway（Gateway 内部存 runner 引用）
gwServer := gateway.New(runner, gateway.WithAllowUsers(...), ...)

// 3. 建 in-process Gateway 客户端（gwclient）
gw := newInProcGatewayClient(gwServer, appName, ...)

// 4. 用 gw 建 Channel（Channel 内部存 gw 引用）
tgChannel := telegram.New(token, botInfo, gw, opts...)
stdinChannel := stdin.NewChannel(deps{Gateway: gw}, ...)

// 5. 启动所有 Channel（每个 goroutine 跑 Run）
go tgChannel.Run(ctx)
go stdinChannel.Run(ctx)

// 6. 启动 HTTP 服务（Gateway 暴露的 Handler）
http.ListenAndServe(":8080", gwServer.Handler())
```

**依赖链**：Channel → gw → gwServer → Runner（单向）

---

## § 7 消息接收全链路

```
[Telegram 用户发 "hello"]
    ↓ getUpdates 拉到这条消息
[Telegram Channel.Run] 死循环
    ↓ onMessage callback 触发
c.handleMessage(ctx, msg)
    ↓ 把 msg 转成 MessageRequest
    ↓ 计算 session_id（dm/thread/topic）
    ↓
c.gw.SendMessage(ctx, MessageRequest{
    Channel: "telegram",
    From:    msg.From.ID,
    Text:    "hello",
    UserID:  msg.From.ID,
})
    ↓ in-process HTTP
[Gateway] handleMessages
    ↓
    1. 检查 allowlist
    2. 计算 session_id
    3. mention 检查
    4. 多模态标准化
    5. 调 runner.Run(ctx, userID, sessionID, msg)
    ↓
[Runner] 真正干活的 AI
    ↓ 流式返回 <-chan *event.Event
[Gateway] 消费 Event 流
    ↓ 转成 MessageResponse
    ↓ 返回给 Channel
[Telegram Channel] 收到 Reply
    ↓ 用 bot API 发回 Telegram
[Telegram 用户看到回复]
```

**对照 stdin channel 核心 30 行**（`plugins/stdin/stdin.go`）：

```go
type channel struct {
    id   string
    gw   GatewayClient    // ← 关键：只依赖接口
    from string
}

func (c *channel) Run(ctx context.Context) error {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        // ...
        rsp, err := c.gw.SendMessage(ctx, MessageRequest{
            Channel: "stdin",
            From:    c.from,
            Text:    text,
            UserID:  c.from,
        })
        fmt.Println(rsp.Reply)
    }
}
```

骨架完全一样：监听 → 转 MessageRequest → 调 gw → 拿 Reply 发回。

---

## § 8 文件接收全链路

**Channel 不真下载字节**——它只传 `FileID`，让 Gateway 来下。

```
[TG 用户] 发 PDF
    ↓
[TG API] getUpdates 拿到 msg.Document.FileID="AgAC..."
    ↓
[Telegram Channel.handleMessage]
    ↓ 不下载字节，只传 FileID
    ↓
c.gw.SendMessage(ctx, MessageRequest{
    Channel: "telegram",
    Text:    msg.Caption,
    ContentParts: [{Type: "file", File: &FilePart{
        Filename: msg.Document.FileName,
        FileID:   msg.Document.FileID,  // 关键：传 ID，不传字节
        Format:   msg.Document.MimeType,
    }}],
})
    ↓ in-proc HTTP
[Gateway.handleMessages]
    ↓ 看到 FileID → 调 DownloadFileByID 下载字节
    ↓ 存到 state_dir/uploads/<sha256>.pdf
    ↓ 设 env 变量：
        OPENCLAW_LAST_PDF_PATH=/path/to/pdf
        OPENCLAW_LAST_UPLOAD_PATH=/path/to/upload
        OPENCLAW_LAST_AUDIO_PATH / IMAGE_PATH / VIDEO_PATH ...
        OPENCLAW_SESSION_UPLOADS_DIR
    ↓ 把路径写进 Agent 的 tool context
    ↓
[Runner] Agent 处理
    工具（如 read_document / read_spreadsheet / exec_command）
    通过 env 变量就能拿到文件路径
```

**关键观察**：
1. Channel 只传 `FileID`，不下字节
2. Gateway 下载后**落盘到 state_dir/uploads/**，后续多轮对话可复用同一份文件
3. **环境变量注入** 让工具不需要知道"刚才传的是什么"，直接读 env 就拿到路径

---

## § 9 文件发送全链路（重点）

**发文件被做成了一个 Tool**。Tool 不直接持有 Channel，必须走 Router 中转。

### 调用链

```
[Agent 推理后说：把 report.pdf 发回去]
    ↓
[Agent 调 message 工具]
    参数：{channel:"telegram", target:"<chatID>", text:"搞定了", files:["/tmp/report.pdf"]}
    ↓
[MessageTool.Call] → router.SendMessage(target, msg)
    ↓
[Router.SendMessage]
    ↓ 查 r.messageSenders["telegram"]
    ↓
[Telegram Channel.SendMessage]   ← 唯一拿到 Channel 的地方
    ↓ 展开目录（如果是目录）
    ↓ detectUploadMode（按扩展名挑 SendDocument/SendPhoto/SendAudio）
    ↓ 调 c.bot.SendDocument（HTTP POST + multipart 上传）
    ↓
[Telegram API] → 用户收到附件
```

### 三层依赖关系（最重要的图）

```
Tool  ──→  Router  ──→  Channel (实现了 MessageSender)
   ↑                        ↑
   │                        │
   └─── Agent 知道有这俩    └─── 平台相关（要换成 Slack 再写一个）
```

### 三层代码骨架

**① Tool 层**（只依赖 Router，不知道有 Channel）：
```go
type MessageTool struct {
    router *outbound.Router   // ← 只持 Router
}

func (t *MessageTool) Call(ctx, args) (any, error) {
    return t.router.SendMessage(ctx, DeliveryTarget{
        Channel: args.Channel,   // "telegram"
        Target:  args.Target,    // "<chatID>"
    }, OutboundMessage{
        Text:  args.Text,
        Files: args.Files,       // ["/tmp/out/report.pdf"]
    })
}
```

**② Router 层**（按 channel 名查 Sender）：
```go
type Router struct {
    mu             sync.RWMutex
    textSenders    map[string]channel.TextSender      // "telegram" → ...
    messageSenders map[string]channel.MessageSender   // "telegram" → ...
}

func (r *Router) SendMessage(ctx, target, msg) error {
    channelID := target.Channel

    r.mu.RLock()
    messageSender := r.messageSenders[channelID]
    r.mu.RUnlock()

    if messageSender != nil {
        return messageSender.SendMessage(ctx, target.Target, msg)
    }
    return fmt.Errorf("outbound: unsupported channel: %s", channelID)
}

// 注册时（NewRuntime 启动时）：
func (r *Router) Register(ch channel.Channel) {
    if sender, ok := ch.(channel.MessageSender); ok {
        r.RegisterMessageSender(sender)
    }
}
```

**③ Channel 层**（具体执行平台 API 调用）：
```go
func (c *Channel) SendMessage(ctx, target, msg) error {
    chatID, threadID, _ := parseTextTarget(target)

    // 把目录展开成多个文件
    files, _ := c.expandTelegramOutboundFiles(ctx, msg.Files)

    // 调 c.bot.SendDocument / SendPhoto / SendAudio ...
    //          ↑ c.bot 是 Channel 结构体的字段（调 Telegram API 的能力）
    for _, file := range files {
        c.sendFile(ctx, chatID, threadID, file, ...)
    }
}
```

### 关键约束

| 顾虑 | OpenClaw 的处理方式 |
|---|---|
| Agent 乱给陌生人发消息？ | 必须传 `target`（chatID）。如果用户没指定，框架从 ctx 注入当前会话 |
| Agent 跨 Channel 乱发？ | Router 按 `channel` 名查 sender。Agent 想"从 TG 发到 Slack"必须显式传 `channel="slack"` |
| Agent 乱发文件到错的 chat？ | Channel 内部还有校验（allowUsers 等） |

**核心原则**：**Tool 跟 Channel 完全不接触**。Tool 只接触 Router。

### 为什么这么设计？

加新 Channel 时：
- ❌ **不要**：Tool 里加 if channel=="slack"
- ✅ **要**：写一个新 Channel 结构体实现 `MessageSender`，在 Router 里 Register

Tool 的代码**永远不需要改**。

---

## § 10 Gateway HTTP 接口设计

OpenClaw 用了**双 endpoint + 配套接口**，客户端主动选。

### 三个路由

```
GET  /healthz                            健康检查
POST /v1/gateway/messages                同步（一次性 JSON 响应）
POST /v1/gateway/messages:stream         SSE 流式
POST /v1/gateway/cancel                  按 request_id 取消运行
GET  /v1/gateway/status?request_id=xxx    按 request_id 查进度
```

### 同步 vs 流式：区别只在"返回时机"

**同步版**（`internal/gateway/server.go:390`）：
```go
func (s *Server) handleMessages(w, r) {
    var req gwproto.MessageRequest
    json.NewDecoder(r.Body).Decode(&req)

    rsp, status := s.ProcessMessage(r.Context(), req)  // 等所有 Event 读完
    s.writeJSON(w, rsp, status)                       // 一次性返回 JSON
}
```

**流式版**（`internal/gateway/stream.go`）：
```go
func (s *Server) handleStream(w, r) {
    w.Header().Set("Content-Type", "text/event-stream")  // SSE
    flusher := w.(http.Flusher)

    events := s.runStreaming(r.Context(), req)
    for ev := range events {
        fmt.Fprintf(w, "data: %s\n\n", ev)  // 一个 Event 一次 SSE
        flusher.Flush()                      // 立刻发出去
    }
}
```

**几乎共享全部逻辑**，只在"消费 Runner Event 流的方式"上分叉。

### 客户端怎么选

| 场景 | 用 | 原因 |
|---|---|---|
| Telegram Bot 收消息 → 处理 → 回复 | **流式** | 想给用户"打字效果"（编辑一条消息反复改） |
| stdin 一行一行 | **同步** | 终端读一行打一行回一行 |
| HTTP webhook 触发任务 | **同步** | 简单 curl 测试方便 |
| Cron 定时任务 | **同步** | 不用给人看，简化处理 |
| A2A 别的 Agent 调用 | **流式** | 可能需要实时反馈 |

**Channel 自己决定**——Telegram Channel 选流式，stdin Channel 选同步。

```go
// gwclient 同一个 Client 的两个方法
rsp, err := c.SendMessage(ctx, req)              // 同步
events, err := c.StreamMessage(ctx, req, opts)   // 流式
```

### 为什么不是"立刻 200 + 后台跑"？

| 方案 | 问题 |
|---|---|
| **A: 立刻 200 + 后台跑 + 第二个接口 SSE** | 多一个 request_id 状态查询机制；前端要维护异步关联 |
| **B: 一个接口 SSE** | 调用方（Telegram Channel）想"等所有结果再发一条消息"时不方便，要写客户端逻辑 |
| **✅ 双 endpoint（OpenClaw）** | 调用方主动选，没有异步关联复杂度，也没有"客户端逻辑要写两遍" |

### /cancel 和 /status 配套使用

```
场景：流式响应跑了 5 分钟还没完
1. 前端发 POST /cancel {"request_id": "xxx"}
2. Gateway 找到 runner.ManagedRunner.Cancel(requestID)
3. Event 流结束，SSE 连接关闭

场景：流式断了想续上 / 想看进度
1. 前端发 GET /status?request_id=xxx
2. Gateway 返回当前状态（还在跑 / 已完成 / 已取消 / 错误）
```

---

## § 11 三条铁律（最容易出错的边界）

### 铁律 1：Tool 不持有 Channel，只持有 Router

```go
// ✅ 正确
type MessageTool struct {
    router *outbound.Router
}

// ❌ 错误
type MessageTool struct {
    telegramChannel *TelegramChannel  // Tool 跟 Channel 耦合死
    slackChannel    *SlackChannel
}
```

### 铁律 2：Channel 同时实现 Channel 和 MessageSender 接口

同一个结构体既能"被动收"（Channel.Run）又能"主动发"（MessageSender.SendMessage），因为它**组合了多种能力**：

```go
type Channel struct {
    bot  botAPI            // ← 能力 1：发 IM API（实现 MessageSender）
    gw   gatewayClient     // ← 能力 2：收消息后转给 Gateway（实现 Channel）
    // ...
}
```

### 铁律 3：Channel 不下载字节，只传 FileID；Gateway 落盘后注入 env

```
[TG Document.FileID="AgAC..."] 
    ↓ Channel 不下载，传 ID
[MessageRequest{ContentParts: [{File: {FileID: "AgAC..."}}]}]
    ↓ Gateway 收到，DownloadFileByID 下字节
[存到 state_dir/uploads/<sha256>.pdf]
    ↓ 设 env 变量
[OPENCLAW_LAST_PDF_PATH=/state_dir/uploads/<sha256>.pdf]
    ↓ Agent 工具能直接读 env 拿路径
[read_document(exec_command) 工具调用]
```

**好处**：
- 文件后续多轮对话可复用（同一 sha256）
- Agent 工具不需要知道"刚才传的是什么"，直接读 env
- 落盘后可以做大小限制、清理、安全检查

---

## 一页纸速查卡

```
┌─────────────────────────────────────────────────────────────────┐
│  Channel        监听 IM 平台       实现 Channel 接口             │
│  Gateway        HTTP 入口          同步 + SSE + cancel + status  │
│  Runner         trpc-agent-go 原生  流式 Event 输出              │
│                                                                  │
│  依赖链：Channel → gw → gwServer → Runner（单向）                │
│                                                                  │
│  Channel 不下载字节，只传 FileID                                 │
│  Gateway 下载 + 落盘 state_dir/uploads + 注入 env 变量          │
│                                                                  │
│  发文件 = Tool（message）→ Router（map）→ Channel（实现 Sender）│
│  Tool 永远不持有 Channel                                         │
│                                                                  │
│  HTTP 双 endpoint：客户端主动选同步 vs 流式                       │
│  /cancel + /status 配套流式使用                                  │
└─────────────────────────────────────────────────────────────────┘
```