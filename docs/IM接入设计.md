# IM 接入设计

> 文本先行，文件后续。

## 核心思想

每个 IM 是一个独立的 goroutine，共享同一个 `r.Run()`。

```
Telegram goroutine        飞书 goroutine         Slack goroutine
     │                        │                       │
     └─── gw.SendText() ──────┼───────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   GatewayClient   │
                    │   (封装 r.Run())  │
                    │                   │
                    │   r.Run(userID,   │
                    │    sessionID,     │
                    │    userMsg)       │
                    │    → 拼 event     │
                    │    → 返回文本     │
                    └───────────────────┘
```

## GatewayClient

```go
type GatewayClient struct {
    runner runner.Runner
}

func (g *GatewayClient) SendText(ctx context.Context, input SendTextInput) (SendTextOutput, error) {
    eventCh, err := g.runner.Run(ctx, input.UserID, input.SessionID, model.NewUserMessage(input.Text))
    if err != nil {
        return SendTextOutput{}, err
    }

    var reply strings.Builder
    for ev := range eventCh {
        if ev.Response != nil && len(ev.Response.Choices) > 0 {
            reply.WriteString(ev.Response.Choices[0].Delta.Content)
        }
    }

    return SendTextOutput{Reply: reply.String()}, nil
}

type SendTextInput struct {
    UserID    string
    SessionID string
    Text      string
}

type SendTextOutput struct {
    Reply string
}
```

## Channel 职责

收消息 → 映射 userID/sessionID → 调 Gateway → 发回复。

```go
// 伪代码
func (c *TelegramChannel) handleMessage(ctx, msg) {
    rsp := c.gw.SendText(ctx, SendTextInput{
        UserID:    "tg:" + msg.From.ID,
        SessionID: "tg:dm:" + msg.From.ID,
        Text:      msg.Text,
    })
    c.bot.SendMessage(msg.Chat.ID, rsp.Reply)
}
```

## 接入飞书就是换文件名

```go
func (c *FeishuChannel) handleMessage(ctx, event) {
    rsp := c.gw.SendText(ctx, SendTextInput{
        UserID:    "fs:" + event.SenderID,
        SessionID: "fs:dm:" + event.SenderID,
        Text:      event.Content.Text,
    })
    c.bot.SendMessage(event.ChatID, rsp.Reply)
}
```

## 会话隔离

| 维度 | 值 | 说明 |
|---|---|---|
| app_name | 进程固定 | 所有人共享 |
| user_id | "渠道:用户ID" | 跨 session 共享 Memory |
| session_id | "渠道:dm|thread:ID" | 每个聊天独立历史 |

### 开发阶段：Web 与 Telegram 共享会话

跑通阶段 Web 和 Telegram 用同一个 userID + sessionID，对话历史互通：

```json
// Web 前端
{ "user_id": "u-alice", "session_id": "u-alice", "message": "..." }

// Telegram handler
gw.SendText(ctx, SendTextInput{
    UserID:    "u-alice",
    SessionID: "u-alice",
    Text:      msg.Text,
})
```

后续加多用户时，再改成渠道前缀区分方式。

## 收消息方式

Telegram 使用长轮询（不需要公网 IP）。offset 持久化到文件即可，不需要数据库：

```
启动 → 读文件拿到 offset → getUpdates(offset)
    ↓
收到消息 → offset = updateID + 1 → 写回文件
    ↓
崩溃重启 → 读文件 → offset 还是对的
```

各 IM 收消息方式不同：

| 平台 | 方式 | 公网要求 |
|------|------|---------|
| Telegram | 长轮询（默认）或 Webhook | 轮询不需要 |
| 飞书 | Webhook | 需要 HTTPS |
| Web 前端 | HTTP POST（框架自带） | 不需要 |

## 文件扩展（后续）

Gateway 返回的 event 包含 tool result。当 tool result 中有文件引用时，Channel 从 sandbox 工作目录读取并发送。

## 实施计划（文本阶段）

### 新增文件

```
trpc-agent-go-demo/
├── gateway/
│   └── gateway.go        ← GatewayClient
├── channel/
│   ├── telegram.go       ← Telegram 长轮询
│   └── channel_test.go   ← 测试
└── main.go               ← 加 5 行启动 telegram
```

### 1. gateway/gateway.go

```go
package gateway

type Client struct {
    runner runner.Runner
}

func NewClient(r runner.Runner) *Client {
    return &Client{runner: r}
}

type SendTextInput struct {
    UserID    string
    SessionID string
    Text      string
}

type SendTextOutput struct {
    Reply string
}

func (c *Client) SendText(ctx context.Context, in SendTextInput) (SendTextOutput, error) {
    eventCh, err := c.runner.Run(ctx, in.UserID, in.SessionID, model.NewUserMessage(in.Text))
    if err != nil {
        return SendTextOutput{}, err
    }

    var reply strings.Builder
    for ev := range eventCh {
        if ev.Response != nil && len(ev.Response.Choices) > 0 {
            reply.WriteString(ev.Response.Choices[0].Delta.Content)
        }
    }

    return SendTextOutput{Reply: reply.String()}, nil
}
```

### 2. channel/telegram.go

依赖：[go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)

```go
type Channel struct {
    bot        *tgbotapi.BotAPI
    gw         *gateway.Client
    offsetFile string
}

func NewChannel(token string, gw *gateway.Client, stateDir string) (*Channel, error) {
    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        return nil, err
    }

    return &Channel{
        bot:        bot,
        gw:         gw,
        offsetFile: filepath.Join(stateDir, "telegram-offset"),
    }, nil
}

func (c *Channel) Run(ctx context.Context) error {
    offset := c.loadOffset()

    cfg := tgbotapi.UpdateConfig{
        Offset:  offset,
        Timeout: 25,
    }

    for {
        if ctx.Err() != nil {
            return ctx.Err()
        }

        updates, err := c.bot.GetUpdates(cfg)
        if err != nil {
            time.Sleep(time.Second)
            continue
        }

        for _, u := range updates {
            if u.Message == nil || u.Message.Chat == nil {
                continue
            }
            if !u.Message.Chat.IsPrivate() {
                continue  // 只做私聊
            }

            go c.handleMessage(ctx, u.Message)

            if u.UpdateID >= offset {
                offset = u.UpdateID + 1
                cfg.Offset = offset
            }
        }
    }
}

func (c *Channel) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
    out, err := c.gw.SendText(ctx, gateway.SendTextInput{
        UserID:    "u-alice",
        SessionID: "u-alice",
}
```

offset 读写的实现略（文件存一个整数）。完整实现加上 context 里的 offset 追踪和错误重试。

### 3. main.go 改动

```go
// 现有代码不动，末尾加：
gw := gateway.NewClient(r)

tg, err := channel.NewTelegramChannel(
    envOr("TELEGRAM_TOKEN", ""),
    gw,
    "./state",
)
if err != nil {
    log.Printf("telegram: %v", err)
} else {
    go func() {
        if err := tg.Run(ctx); err != nil {
            log.Printf("telegram: %v", err)
        }
    }()
    log.Printf("Telegram bot listening...")
}
```

### 依赖

```
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
```

### 总代码量预估

- `gateway/gateway.go`: ~50 行
- `channel/telegram.go`: ~150 行
- `main.go` 改动: ~15 行
- **合计新代码**: ~200 行
