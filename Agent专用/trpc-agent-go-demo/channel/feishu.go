package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"demo/gateway"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im"
	larkimv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

// FeishuChannel handles Feishu messages via WebSocket long connection.
type FeishuChannel struct {
	appID     string
	appSecret string
	gw        *gateway.Client
	imService *larkim.Service
}

func NewFeishuChannel(appID, appSecret string, gw *gateway.Client) (*FeishuChannel, error) {
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("feishu: APP_ID and APP_SECRET are required")
	}

	// Use the full SDK client — it initializes serialization, cache, HTTP client, etc.
	client := lark.NewClient(appID, appSecret)

	return &FeishuChannel{
		appID:     appID,
		appSecret: appSecret,
		gw:        gw,
		imService: client.Im,
	}, nil
}

// Run starts the WebSocket client. Blocks until connection drops or ctx is cancelled.
func (c *FeishuChannel) Run(ctx context.Context) error {
	eventHandler := dispatcher.NewEventDispatcher("", "")

	eventHandler.OnP2MessageReceiveV1(func(ctx context.Context, event *larkimv1.P2MessageReceiveV1) error {
		go c.handleMessage(ctx, event)
		return nil
	})

	cli := ws.NewClient(c.appID, c.appSecret,
		ws.WithEventHandler(eventHandler),
	)

	log.Printf("Feishu bot running...")
	return cli.Start(ctx)
}

type feishuTextContent struct {
	Text string `json:"text"`
}

func (c *FeishuChannel) handleMessage(ctx context.Context, event *larkimv1.P2MessageReceiveV1) {
	if event.Event == nil || event.Event.Message == nil {
		return
	}

	msg := event.Event.Message

	// Only handle text messages
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return
	}

	var content feishuTextContent
	if msg.Content == nil {
		return
	}
	if err := json.Unmarshal([]byte(*msg.Content), &content); err != nil {
		return
	}

	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}

	sender := ""
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		sender = *event.Event.Sender.SenderId.OpenId
	}
	log.Printf("[feishu] %s: %s", sender, content.Text)

	reply, err := c.gw.SendText(ctx, gateway.SendTextInput{
		UserID:    "u-alice",
		SessionID: "u-alice",
		Text:      content.Text,
	})
	if err != nil {
		log.Printf("gateway error: %v", err)
		c.sendReply(ctx, chatID, "出错了，稍后再试")
		return
	}

	if reply != "" {
		c.sendReply(ctx, chatID, reply)
	}
}

func (c *FeishuChannel) sendReply(ctx context.Context, chatID, text string) {
	contentBytes, _ := json.Marshal(feishuTextContent{Text: text})

	req := larkimv1.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkimv1.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(string(contentBytes)).
			Build()).
		Build()

	if _, err := c.imService.Message.Create(ctx, req); err != nil {
		log.Printf("feishu send: %v", err)
	}
}
