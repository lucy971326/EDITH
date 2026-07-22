package channel

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"demo/gateway"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Channel handles Telegram messages via long polling.
type Channel struct {
	bot        *tgbotapi.BotAPI
	gw         *gateway.Client
	offsetFile string
}

func NewChannel(token string, gw *gateway.Client, stateDir string, proxyURL string) (*Channel, error) {
	httpClient := &http.Client{}
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("telegram proxy url: %w", err)
		}
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
		log.Printf("Telegram bot using proxy: %s", proxyURL)
	}

	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("telegram state dir: %w", err)
	}

	return &Channel{
		bot:        bot,
		gw:         gw,
		offsetFile: filepath.Join(stateDir, "telegram-offset"),
	}, nil
}

// Run starts the long polling loop. Blocks until ctx is cancelled.
func (c *Channel) Run(ctx context.Context) error {
	log.Printf("Telegram bot [@%s] running...", c.bot.Self.UserName)

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
			if strings.Contains(err.Error(), "canceled") {
				continue
			}
			log.Printf("telegram getUpdates: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, u := range updates {
			if u.Message == nil || u.Message.Chat == nil {
				continue
			}
			if !u.Message.Chat.IsPrivate() {
				continue // only DMs for now
			}

			go c.handleMessage(ctx, u.Message)

			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
				cfg.Offset = offset
				c.saveOffset(offset)
			}
		}
	}
}

func (c *Channel) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	log.Printf("[tg] %s: %s", msg.From.UserName, msg.Text)

	reply, err := c.gw.SendText(ctx, gateway.SendTextInput{
		UserID:    "u-alice",
		SessionID: "u-alice",
		Text:      msg.Text,
	})
	if err != nil {
		log.Printf("gateway error: %v", err)
		c.sendReply(msg.Chat.ID, "❌ 出错了，稍后再试")
		return
	}

	if reply != "" {
		c.sendReply(msg.Chat.ID, reply)
	}
}

func (c *Channel) sendReply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := c.bot.Send(msg); err != nil {
		log.Printf("telegram send: %v", err)
	}
}

// ---- offset persistence ----

func (c *Channel) loadOffset() int {
	data, err := os.ReadFile(c.offsetFile)
	if err != nil {
		return 0
	}
	offset, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return offset
}

func (c *Channel) saveOffset(offset int) {
	if err := os.WriteFile(c.offsetFile, []byte(strconv.Itoa(offset)), 0644); err != nil {
		log.Printf("telegram save offset: %v", err)
	}
}
