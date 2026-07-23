package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"demo/sandbox"

	"demo/channel"
	"demo/gateway"

	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	memorysqlite "trpc.group/trpc-go/trpc-agent-go/memory/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// ============================================================================
// 前端协议事件结构体（trpc-agent-go Event → 前端 SSE）
// ============================================================================

type FrontendEvent struct {
	Type       string `json:"type"`                  // "text" | "reasoning" | "tool_call" | "tool_result" | "error" | "done"
	RequestID  string `json:"request_id"`            // 请求唯一 ID
	ResponseID string `json:"response_id,omitempty"` // 本次 LLM Response 的唯一 ID：文本/思考归属到对应回复气泡
	Text       string `json:"text,omitempty"`        // text 事件：流式文本片段
	Thinking   string `json:"thinking,omitempty"`    // reasoning 事件：推理过程
	ToolID     string `json:"id,omitempty"`          // tool_call/tool_result：工具调用 ID
	ToolName   string `json:"name,omitempty"`        // tool_call/tool_result：工具名
	Arguments  string `json:"arguments,omitempty"`   // tool_call：JSON 字符串，前端需 JSON.parse
	Result     any    `json:"result,omitempty"`      // tool_result：工具返回（JSON）
	Message    string `json:"message,omitempty"`     // error：错误文案
	Usage      *Usage `json:"usage,omitempty"`       // done：token 用量
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ============================================================================
// 模型列表
// ============================================================================

type ModelInfo struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Vision bool   `json:"vision"`
}

// 模型能力（视觉支持等）
var modelCaps = map[string]bool{
	"deepseek-v4-flash": false,
	"deepseek-v4-pro":   false,
	"MiniMax-M3":        true,
}

// ============================================================================
// 工具数据类型
// ============================================================================

type currentTimeArgs struct {
	Timezone string `json:"timezone" jsonschema:"description=IANA 时区名，例如 Asia/Shanghai 或 America/New_York，留空用北京时间"`
}
type currentTimeResult struct {
	Location    string `json:"location"`
	Time        string `json:"time"`
	Weekday     string `json:"weekday"`
	Timezone    string `json:"timezone"`
	UnixSeconds int64  `json:"unix_seconds"`
}

// ============================================================================
// 工具实现
// ============================================================================

func getCurrentTime(ctx context.Context, args currentTimeArgs) (currentTimeResult, error) {
	loc, err := time.LoadLocation(args.Timezone)
	if err != nil || loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	return currentTimeResult{
		Location:    loc.String(),
		Time:        now.Format("2006-01-02 15:04:05 MST"),
		Weekday:     now.Weekday().String(),
		Timezone:    now.Location().String(),
		UnixSeconds: now.Unix(),
	}, nil
}

// ============================================================================
// 辅助
// ============================================================================

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newBackendProvider selects how session workspaces get their execution backend.
func newBackendProvider(db *sql.DB) sandbox.BackendProvider {
	if os.Getenv("SANDBOX_MODE") == "e2b" {
		provider, err := sandbox.NewE2BProvider(db, sandbox.E2BProviderOptions{
			APIKey:   envOr("E2B_API_KEY", ""),
			Domain:   envOr("E2B_DOMAIN", ""),
			Template: envOr("E2B_TEMPLATE", "base"),
			Timeout:  10 * time.Minute,
		})
		if err != nil {
			log.Fatalf("new E2B backend provider: %v", err)
		}
		return provider
	}
	provider, err := sandbox.NewLocalProvider("./workspace")
	if err != nil {
		log.Fatalf("new local backend provider: %v", err)
	}
	return provider
}

// ============================================================================
// main：组装一切
// ============================================================================

func main() {
	ctx := context.Background()

	// ----- 1. Models（预注册多个，请求时按名称切换）-----
	// 每个模型显式指定 API Key + Base URL，优先读专用环境变量
	deepseekKey := envOr("DEEPSEEK_API_KEY", "sk-48273bd3d17d486a861b61a06be381e1")
	deepseekURL := envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")
	minimaxKey := envOr("MINIMAX_API_KEY", "sk-cp-8Bnw6tM8ZV3JZH74o-c2j-UOoYk3ktO_FDFQGAzNn76Qk4ZLX1NNI492sWI0YwIRqf_NC7kyHel8CrJe7k_hZI2sboFaEp_gAEl8WEgoCHsUbwGJoWC1h8o")
	minimaxURL := envOr("MINIMAX_BASE_URL", "https://api.minimaxi.com/v1")

	models := map[string]model.Model{
		// ---- DeepSeek ----
		"deepseek-v4-flash": openai.New("deepseek-v4-flash",
			openai.WithAPIKey(deepseekKey),
			openai.WithBaseURL(deepseekURL),
		),
		"deepseek-v4-pro": openai.New("deepseek-v4-pro",
			openai.WithAPIKey(deepseekKey),
			openai.WithBaseURL(deepseekURL),
		),
		// ---- OpenAI ----
		"MiniMax-M3": openai.New("MiniMax-M3",
			openai.WithAPIKey(minimaxKey),
			openai.WithBaseURL(minimaxURL),
			openai.WithExtraFields(map[string]any{"reasoning_split": true}),
		),
	}

	// ----- 2. Session -----
	sessionDB, _ := sql.Open("sqlite3", "file:demo.db?_busy_timeout=5000&_journal_mode=WAL")
	sessionService, _ := sessionsqlite.NewService(sessionDB, sessionsqlite.WithSessionEventLimit(500))
	defer sessionService.Close()

	// ----- 3. Memory -----
	memoryDB, _ := sql.Open("sqlite3", "file:demo.db?_busy_timeout=5000&_journal_mode=WAL")
	memoryService, _ := memorysqlite.NewService(memoryDB, memorysqlite.WithMemoryLimit(1000))
	defer memoryService.Close()

	// ----- 4. Tools -----
	timeTool := function.NewFunctionTool(
		getCurrentTime,
		function.WithName("get_current_time"),
		function.WithDescription("查询指定时区当前的本地时间。不传 timezone 默认用北京时间。"),
	)
	tools := append([]tool.Tool{timeTool}, memoryService.Tools()...)

	// ----- 5. MCP ToolSets -----
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN is required")
	}
	githubToolSet := mcp.NewMCPToolSet(
		mcp.ConnectionConfig{
			Transport: "streamable_http",
			ServerURL: "https://api.githubcopilot.com/mcp/",
			Timeout:   30 * time.Second,
			Headers:   map[string]string{"Authorization": "Bearer " + githubToken, "X-MCP-Toolsets": "default"},
		},
		mcp.WithName("github"),
	)
	defer githubToolSet.Close()
	githubToolSet.Init(ctx)

	// Sandbox ToolSet — resolves an isolated backend from each Runner invocation.
	backendProvider := newBackendProvider(memoryDB)
	sandboxTS := sandbox.NewToolSet(backendProvider)
	defer sandboxTS.Close()

	// ----- 6. Agent -----
	llmAgent := llmagent.New(
		"assistant",
		llmagent.WithModels(models),
		llmagent.WithModel(models["MiniMax-M3"]), // 默认模型
		llmagent.WithInstruction(loadSystemPrompt()),
		llmagent.WithGenerationConfig(model.GenerationConfig{Stream: true}),
		llmagent.WithTools(tools),
		llmagent.WithToolSets([]tool.ToolSet{githubToolSet, sandboxTS}),
		llmagent.WithPreloadMemory(10),
		llmagent.WithToolCallRetryPolicy(&tool.RetryPolicy{
			MaxAttempts: 3, InitialInterval: 200 * time.Millisecond, BackoffFactor: 2.0,
		}),
	)

	// ----- 7. Runner -----
	r := runner.NewRunner(
		"demo-app",
		llmAgent,
		runner.WithSessionService(sessionService),
		runner.WithMemoryService(memoryService),
	)
	defer r.Close()

	// ----- 8. Gateway + Telegram -----
	gw := gateway.NewClient(r)

	// TelegramService 管理用户 Bot，并将 Webhook 消息经 Gateway 交给 Agent 后回复 Telegram。
	telegramService, err := channel.NewTelegramService(
		gw,
		channel.TelegramConfig{
			WebhookBaseURL: os.Getenv("TELEGRAM_WEBHOOK_BASE_URL"),
			ProxyURL:       os.Getenv("TELEGRAM_PROXY"),
		})
	if err != nil {
		log.Fatalf("telegram service: %v", err)
	}

	// ----- 9. HTTP Server -----
	mux := http.NewServeMux()
	mux.HandleFunc("/telegram/configure", telegramService.HandleConfigure)
	mux.HandleFunc("POST /webhook/telegram/{routeKey}", telegramService.HandleWebhook)

	// 原生 SSE handler — POST JSON body → SSE stream
	mux.HandleFunc("POST /stream", sseHandler(r))

	// 模型列表（从 models map 动态生成，单一真理来源）
	mux.HandleFunc("GET /models", func(w http.ResponseWriter, _ *http.Request) {
		list := make([]ModelInfo, 0, len(models))
		for key, m := range models {
			list = append(list, ModelInfo{ID: key, Label: m.Info().Name, Vision: modelCaps[key]})
		}
		writeJSON(w, list)
	})

	// 会话列表
	mux.HandleFunc("GET /sessions", sessionsHandler(sessionService))

	// 会话历史
	mux.HandleFunc("GET /sessions/{sessionID}", sessionHistoryHandler(sessionService))

	log.Printf("Agent API : http://%s/stream", envOr("ADDR", "127.0.0.1:8080"))
	http.ListenAndServe(envOr("ADDR", "127.0.0.1:8080"), mux)
}

// ============================================================================
// ============================================================================
// SSE Handler：POST JSON body → SSE stream
// ============================================================================

type ImageInput struct {
	Data   string `json:"data"`   // base64 编码的图片数据（不含 data:xxx;base64, 前缀）
	Format string `json:"format"` // "png" | "jpeg" | "webp" | "gif"
}

type StreamInput struct {
	UserID    string       `json:"user_id"`
	SessionID string       `json:"session_id"`
	Message   string       `json:"message"`
	Model     string       `json:"model,omitempty"`  // 可选：指定模型名，空则用默认
	Images    []ImageInput `json:"images,omitempty"` // 可选：图片列表
}

func sseHandler(r runner.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// 解析 POST body
		var input StreamInput
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"message\":\"%s\"}\n\n", err.Error())
			flusher.Flush()
			return
		}
		if strings.TrimSpace(input.UserID) == "" {
			writeSSE(w, "error", FrontendEvent{Type: "error", Message: "user_id is required"})
			flusher.Flush()
			return
		}

		requestID := uuid.NewString()
		runOpts := []agent.RunOption{
			agent.WithRequestID(requestID),
			agent.WithStream(true),
		}
		if input.Model != "" {
			runOpts = append(runOpts, agent.WithModelName(input.Model))
		}

		// 构造用户消息：纯文本 or 多模态（文本+图片）
		var userMsg model.Message
		if len(input.Images) > 0 {
			userMsg = model.Message{Role: model.RoleUser, Content: input.Message}
			for _, img := range input.Images {
				data, err := base64.StdEncoding.DecodeString(img.Data)
				if err != nil {
					writeSSE(w, "error", FrontendEvent{Type: "error", RequestID: requestID, Message: "invalid base64 image: " + err.Error()})
					flusher.Flush()
					return
				}
				userMsg.AddImageData(data, "", img.Format)
			}
		} else {
			userMsg = model.NewUserMessage(input.Message)
		}

		eventCh, err := r.Run(
			req.Context(),
			input.UserID,
			input.SessionID,
			userMsg,
			runOpts...,
		)
		if err != nil {
			writeSSE(w, "error", FrontendEvent{Type: "error", RequestID: requestID, Message: err.Error()})
			flusher.Flush()
			return
		}

		for ev := range eventCh {
			if ev.Error != nil {
				writeSSE(w, "error", FrontendEvent{Type: "error", RequestID: requestID, Message: ev.Error.Message})
				flusher.Flush()
				continue
			}

			if ev.IsRunnerCompletion() {
				var usage *Usage
				if ev.Response != nil && ev.Response.Usage != nil {
					usage = &Usage{
						PromptTokens:     ev.Response.Usage.PromptTokens,
						CompletionTokens: ev.Response.Usage.CompletionTokens,
						TotalTokens:      ev.Response.Usage.TotalTokens,
					}
				}
				writeSSE(w, "done", FrontendEvent{Type: "done", RequestID: requestID, Usage: usage})
				flusher.Flush()
				return
			}

			if ev.Response == nil || len(ev.Response.Choices) == 0 {
				continue
			}

			choice := ev.Response.Choices[0]

			if choice.Delta.ReasoningContent != "" {
				writeSSE(w, "reasoning", FrontendEvent{Type: "reasoning", RequestID: requestID, ResponseID: ev.Response.ID, Thinking: choice.Delta.ReasoningContent})
				flusher.Flush()
			}

			if choice.Delta.Content != "" {
				writeSSE(w, "text", FrontendEvent{Type: "text", RequestID: requestID, ResponseID: ev.Response.ID, Text: choice.Delta.Content})
				flusher.Flush()
			}

			for _, call := range choice.Message.ToolCalls {
				writeSSE(w, "tool_call", FrontendEvent{Type: "tool_call", RequestID: requestID, ResponseID: ev.Response.ID, ToolID: call.ID, ToolName: call.Function.Name, Arguments: string(call.Function.Arguments)})
				flusher.Flush()
			}

			// 遍历所有 Choices（并行工具结果可能在多个 Choice 里）
			for _, c := range ev.Response.Choices {
				if c.Message.Role == model.RoleTool && c.Message.ToolID != "" {
					var result any
					json.Unmarshal([]byte(c.Message.Content), &result)
					writeSSE(w, "tool_result", FrontendEvent{Type: "tool_result", RequestID: requestID, ToolID: c.Message.ToolID, ToolName: c.Message.ToolName, Result: result})
					flusher.Flush()
				}
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
