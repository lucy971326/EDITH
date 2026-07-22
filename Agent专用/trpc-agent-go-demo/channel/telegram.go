package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"demo/gateway"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type TelegramConfig struct {
	WebhookBaseURL string
	ProxyURL       string
}

// TelegramService 管理所有用户配置的 Telegram Bot。
type TelegramService struct {
	gateway        *gateway.Client
	webhookBaseURL string
	httpClient     *http.Client

	botsMu   sync.RWMutex // 保护 byUser 和 byRoute 的读写
	changeMu sync.Mutex   // 保证“配置/替换/删除 Bot”整套流程不会互相打架
	byUser   map[string]*telegramBot
	byRoute  map[string]*telegramBot
}

// telegramBot 保存一个用户 Bot 的归属和 Telegram 客户端。
type telegramBot struct {
	ownerUserID string
	routeKey    string
	client      *tgbotapi.BotAPI
}

type TelegramStatus struct {
	Connected bool   `json:"connected"`
	Username  string `json:"username,omitempty"`
}

type telegramConfigureInput struct {
	UserID   string `json:"user_id"`
	BotToken string `json:"bot_token"`
}

func NewTelegramService(gw *gateway.Client, cfg TelegramConfig) (*TelegramService, error) {
	httpClient := &http.Client{Timeout: 35 * time.Second}
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("telegram proxy url: %w", err)
		}
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		log.Printf("Telegram bot using proxy: %s", cfg.ProxyURL)
	}

	return &TelegramService{
		gateway:        gw,
		webhookBaseURL: strings.TrimRight(cfg.WebhookBaseURL, "/"),
		httpClient:     httpClient,
		byUser:         make(map[string]*telegramBot),
		byRoute:        make(map[string]*telegramBot),
	}, nil
}

// Configure 验证 Token、注册 Webhook，并替换该用户原有的 Bot。
func (s *TelegramService) Configure(userID, token string) (TelegramStatus, error) {
	userID = strings.TrimSpace(userID)
	token = strings.TrimSpace(token)
	if userID == "" || token == "" {
		return TelegramStatus{}, fmt.Errorf("user_id and bot_token are required")
	}
	if s.webhookBaseURL == "" {
		return TelegramStatus{}, fmt.Errorf("TELEGRAM_WEBHOOK_BASE_URL is required")
	}

	bot, err := s.connectBot(userID, token)
	if err != nil {
		return TelegramStatus{}, err
	}

	// Bot 的替换过程需要串行，普通消息仍可并发处理。
	s.changeMu.Lock()
	defer s.changeMu.Unlock()

	if owner := s.ownerOfBot(bot.client.Self.ID); owner != "" && owner != userID {
		return TelegramStatus{}, fmt.Errorf("this Telegram Bot is already connected by another user")
	}

	// 先登记路由，避免 setWebhook 成功后第一条消息找不到 Bot。
	s.botsMu.Lock()
	s.byRoute[bot.routeKey] = bot
	s.botsMu.Unlock()

	webhookURL := s.webhookBaseURL + "/webhook/telegram/" + bot.routeKey
	if err := bot.setWebhook(webhookURL); err != nil {
		s.botsMu.Lock()
		delete(s.byRoute, bot.routeKey)
		s.botsMu.Unlock()
		return TelegramStatus{}, fmt.Errorf("register telegram webhook: %w", err)
	}

	s.botsMu.Lock()
	oldBot := s.byUser[userID]
	s.byUser[userID] = bot
	if oldBot != nil {
		delete(s.byRoute, oldBot.routeKey)
	}
	s.botsMu.Unlock()

	if oldBot != nil && oldBot.client.Self.ID != bot.client.Self.ID {
		if err := oldBot.deleteWebhook(); err != nil {
			log.Printf("telegram remove old webhook: %v", err)
		}
	}

	return TelegramStatus{Connected: true, Username: bot.client.Self.UserName}, nil
}

func (s *TelegramService) Status(userID string) TelegramStatus {
	s.botsMu.RLock()
	bot := s.byUser[strings.TrimSpace(userID)]
	s.botsMu.RUnlock()
	if bot == nil {
		return TelegramStatus{Connected: false}
	}
	return TelegramStatus{Connected: true, Username: bot.client.Self.UserName}
}

func (s *TelegramService) Remove(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}

	s.changeMu.Lock()
	defer s.changeMu.Unlock()

	s.botsMu.Lock()
	bot := s.byUser[userID]
	if bot != nil {
		delete(s.byUser, userID)
		delete(s.byRoute, bot.routeKey)
	}
	s.botsMu.Unlock()

	if bot == nil {
		return false
	}
	if err := bot.deleteWebhook(); err != nil {
		log.Printf("telegram delete webhook: %v", err)
	}
	return true
}

func (s *TelegramService) connectBot(userID, token string) (*telegramBot, error) {
	client, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, s.httpClient)
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}
	return &telegramBot{
		ownerUserID: userID,
		routeKey:    uuid.NewString(),
		client:      client,
	}, nil
}

func (s *TelegramService) ownerOfBot(botID int64) string {
	s.botsMu.RLock()
	defer s.botsMu.RUnlock()
	for userID, bot := range s.byUser {
		if bot.client.Self.ID == botID {
			return userID
		}
	}
	return ""
}

// HandleConfigure 处理前端的连接、状态查询和断开请求。
func (s *TelegramService) HandleConfigure(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input telegramConfigureInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeTelegramError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		status, err := s.Configure(input.UserID, input.BotToken)
		if err != nil {
			writeTelegramError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeTelegramJSON(w, http.StatusOK, status)

	case http.MethodGet:
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			writeTelegramError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		writeTelegramJSON(w, http.StatusOK, s.Status(userID))

	case http.MethodDelete:
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			writeTelegramError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		if !s.Remove(userID) {
			writeTelegramError(w, http.StatusNotFound, "telegram bot not found")
			return
		}
		writeTelegramJSON(w, http.StatusOK, TelegramStatus{Connected: false})

	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleWebhook 找到对应 Bot，立即确认收件，再在后台运行 Agent。
func (s *TelegramService) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.botsMu.RLock()
	bot := s.byRoute[r.PathValue("routeKey")]
	s.botsMu.RUnlock()
	if bot == nil {
		http.NotFound(w, r)
		return
	}

	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid telegram update", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

	message := update.Message
	if message == nil || message.Chat == nil || !message.Chat.IsPrivate() || strings.TrimSpace(message.Text) == "" {
		return
	}

	baseCtx := context.WithoutCancel(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
		defer cancel()
		s.handleMessage(ctx, bot, message)
	}()
}

func (s *TelegramService) handleMessage(ctx context.Context, bot *telegramBot, message *tgbotapi.Message) {
	from := "unknown"
	if message.From != nil {
		from = message.From.UserName
	}
	log.Printf("[telegram] %s: %s", from, message.Text)

	sessionID := "telegram:dm:" + strconv.FormatInt(message.Chat.ID, 10)
	reply, err := s.gateway.SendText(ctx, gateway.SendTextInput{
		UserID:    bot.ownerUserID,
		SessionID: sessionID,
		Text:      message.Text,
	})
	if err != nil {
		log.Printf("gateway error: %v", err)
		bot.sendReply(message.Chat.ID, "❌ 出错了，稍后再试")
		return
	}
	if reply != "" {
		bot.sendReply(message.Chat.ID, reply)
	}
}

func (b *telegramBot) setWebhook(webhookURL string) error {
	config, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		return err
	}
	_, err = b.client.Request(config)
	return err
}

func (b *telegramBot) deleteWebhook() error {
	_, err := b.client.Request(tgbotapi.DeleteWebhookConfig{})
	return err
}

func (b *telegramBot) sendReply(chatID int64, text string) {
	message := tgbotapi.NewMessage(chatID, text)
	if _, err := b.client.Send(message); err != nil {
		log.Printf("telegram send: %v", err)
	}
}

func writeTelegramJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("telegram response: %v", err)
	}
}

func writeTelegramError(w http.ResponseWriter, statusCode int, message string) {
	writeTelegramJSON(w, statusCode, map[string]string{"error": message})
}
