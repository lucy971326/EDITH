package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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
	filetool "trpc.group/trpc-go/trpc-agent-go/tool/file"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// ============================================================================
// 前端协议事件结构体（trpc-agent-go Event → 前端 SSE）
// ============================================================================

type FrontendEvent struct {
	Type      string `json:"type"`                // "text" | "tool_call" | "tool_result" | "error" | "done"
	RequestID string `json:"request_id"`          // 请求唯一 ID
	Text      string `json:"text,omitempty"`      // text 事件：流式文本片段
	Thinking  string `json:"thinking,omitempty"`  // reasoning 事件：推理过程
	ToolID    string `json:"id,omitempty"`        // tool_call/tool_result：工具调用 ID
	ToolName  string `json:"name,omitempty"`      // tool_call/tool_result：工具名
	Arguments string `json:"arguments,omitempty"` // tool_call：JSON 字符串，前端需 JSON.parse
	Result    any    `json:"result,omitempty"`    // tool_result：工具返回（JSON）
	Message   string `json:"message,omitempty"`   // error：错误文案
	Usage     *Usage `json:"usage,omitempty"`     // done：token 用量
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
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

// ============================================================================
// main：组装一切
// ============================================================================

func main() {
	ctx := context.Background()

	// ----- 1. Model -----
	llm := openai.New(envOr("MODEL_NAME", "deepseek-v4-flash"),
		openai.WithExtraFields(map[string]any{"reasoning_split": true}),
	)

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

	// File operations ToolSet
	fileToolSet, err := filetool.NewToolSet(
		filetool.WithBaseDir("./workspace"),
	)
	if err != nil {
		log.Fatalf("new file toolset: %v", err)
	}
	defer fileToolSet.Close()

	// ----- 6. Agent -----
	llmAgent := llmagent.New(
		"assistant",
		llmagent.WithModel(llm),
		llmagent.WithInstruction(
			"你叫小天，用户的时间助手。\n"+
				"规则：查询 GitHub 前先确认 owner/repo，不要猜测。简洁回复。",
		),
		llmagent.WithGenerationConfig(model.GenerationConfig{Stream: true}),
		llmagent.WithTools(tools),
		llmagent.WithToolSets([]tool.ToolSet{githubToolSet, fileToolSet}),
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

	// ----- 8. HTTP Server -----
	mux := http.NewServeMux()

	// 原生 SSE handler — POST JSON body → SSE stream
	mux.HandleFunc("POST /stream", sseHandler(r))

	log.Printf("Agent API : http://%s/stream", envOr("ADDR", "127.0.0.1:8080"))
	http.ListenAndServe(envOr("ADDR", "127.0.0.1:8080"), mux)
}

// ============================================================================
// ============================================================================
// SSE Handler：POST JSON body → SSE stream
// ============================================================================

type StreamInput struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
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

		requestID := uuid.NewString()
		eventCh, err := r.Run(req.Context(), input.UserID, input.SessionID, model.NewUserMessage(input.Message),
			agent.WithRequestID(requestID),
			agent.WithStream(true),
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
				writeSSE(w, "reasoning", FrontendEvent{Type: "reasoning", RequestID: requestID, Thinking: choice.Delta.ReasoningContent})
				flusher.Flush()
			}

			if choice.Delta.Content != "" {
				writeSSE(w, "text", FrontendEvent{Type: "text", RequestID: requestID, Text: choice.Delta.Content})
				flusher.Flush()
			}

			for _, call := range choice.Message.ToolCalls {
				writeSSE(w, "tool_call", FrontendEvent{Type: "tool_call", RequestID: requestID, ToolID: call.ID, ToolName: call.Function.Name, Arguments: string(call.Function.Arguments)})
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
