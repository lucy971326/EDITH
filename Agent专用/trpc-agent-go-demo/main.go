package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload" // 启动时自动加载 .env
	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	memorysqlite "trpc.group/trpc-go/trpc-agent-go/memory/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/server/agui"
	aguiadapter "trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/claudecode"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// 全局 session 标识（命令行 / AG-UI Server 共用）
var (
	sessionUserID = envOr("AGUI_USER_ID", "u-alice")
	sessionAppName = envOr("AGUI_APP_NAME", "demo-app")
)

// tavilyAPIKey 由 .env 注入（TAVILY_API_KEY）。若为空则跳过 Tavily 工具。
func tavilyAPIKey() string { return os.Getenv("TAVILY_API_KEY") }

// 天气工具的 Mock 数据（包级别变量，便于 weatherTool 闭包直接引用）。
var (
	knownCities = map[string]weatherResult{
		"杭州": {Location: "杭州", Weather: "多云转晴", Temperature: "26°C"},
		"北京": {Location: "北京", Weather: "晴", Temperature: "30°C"},
		"上海": {Location: "上海", Weather: "小雨", Temperature: "24°C"},
		"深圳": {Location: "深圳", Weather: "雷阵雨", Temperature: "31°C"},
	}
	conditions = []string{"晴", "多云", "阴", "小雨", "中雨", "雷阵雨"}
)

func main() {
	ctx := context.Background()

	// ============ 1. Model（OpenAI 兼容协议，DeepSeek/GPT 都用这个） ============
	modelName := envOr("MODEL_NAME", "deepseek-v4-flash")
	llm := openai.New(modelName,
		// # NOTO: M3 兼容 OpenAI 端点时默认走「原生格式」，
		// thinking 会被包成  ̶t̶h̶i̶n̶k̶... ̶/̶t̶h̶i̶n̶k̶  嵌进 content。
		// 加 reasoning_split:true → M3 改走「友好格式」，
		// 把 thinking 单独放到 reasoning_details 字段，
		// 框架流式收集器才能正确分离 reasoning 与正文。
		openai.WithExtraFields(map[string]any{
			"reasoning_split": true,
		}),
	)

	// ============ 2. Session ============
	sessionDB, err := sql.Open("sqlite3", "file:demo.db?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open session db: %v", err)
	}
	sessionService, err := sessionsqlite.NewService(sessionDB, sessionsqlite.WithSessionEventLimit(500))
	if err != nil {
		log.Fatalf("new session service: %v", err)
	}
	defer sessionService.Close()

	// ============ 3. Memory ============
	memoryDB, err := sql.Open("sqlite3", "file:demo.db?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open memory db: %v", err)
	}
	memoryService, err := memorysqlite.NewService(memoryDB, memorysqlite.WithMemoryLimit(1000))
	if err != nil {
		log.Fatalf("new memory service: %v", err)
	}
	defer memoryService.Close()

	// ============ 5. Tools ============
	weatherTool := function.NewFunctionTool(
		getWeather,
		function.WithName("get_weather"),
		function.WithDescription("查询指定城市的天气（Mock 数据，仅用于 Demo）"),
	)
	currentTimeTool := function.NewFunctionTool(
		getCurrentTime,
		function.WithName("get_current_time"),
		function.WithDescription("查询指定时区当前的本地时间。不传 timezone 默认用北京时间。"),
	)
	tools := append([]tool.Tool{weatherTool, currentTimeTool}, memoryService.Tools()...)

	// Claude Code ToolSet:接本地 claude CLI,暴露 Read/Write/Edit/Bash/Grep/Glob 等 12 个工具。
	// baseDir 把工具作用范围限制到 workspace,避免 Agent 误改 demo 自己。
	claudeToolSet, err := claudecode.NewToolSet(
		claudecode.WithBaseDir("/Users/lucy/Documents/EDITH/workspace"),
	)
	if err != nil {
		log.Fatalf("new claudecode toolset: %v", err)
	}
	defer claudeToolSet.Close()

	// Tavily MCP ToolSet:接远程 https://mcp.tavily.com/mcp/?tavilyApiKey=...
	// 只暴露搜索 + 抓取两个工具，避免模型被十几种工具选择困惑。
	// 注意：mcp.NewMCPToolSet 只返回一个值（不像 claudecode 返回 (ToolSet, error)）。
	tavilyToolSet := mcp.NewMCPToolSet(
		mcp.ConnectionConfig{
			Transport: "streamable_http",
			ServerURL: "https://mcp.tavily.com/mcp/?tavilyApiKey=" + tavilyAPIKey(),
			Timeout:   30 * time.Second,
		},
		mcp.WithToolFilterFunc(tool.NewIncludeToolNamesFilter("tavily_search", "tavily_extract")),
	)
	defer tavilyToolSet.Close()
	// Init 提前发现连接失败(否则要等模型实际调用才报错)
	if err := tavilyToolSet.Init(ctx); err != nil {
		log.Fatalf("init tavily toolset: %v", err)
	}

	// GitHub 官方远程 MCP ToolSet：读取仓库、Issue、PR 和 Actions。
	// Token 使用 Fine-grained PAT，并通过环境变量注入，禁止把真实 Token 写进代码或 README。
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN is required (use a fine-grained GitHub PAT)")
	}
	githubToolSet := mcp.NewMCPToolSet(
		mcp.ConnectionConfig{
			Transport: "streamable_http",
			ServerURL: "https://api.githubcopilot.com/mcp/",
			Timeout:   30 * time.Second,
			Headers: map[string]string{
				"Authorization":  "Bearer " + githubToken,
				"X-MCP-Toolsets": "default",
				// 第一阶段只开放查询能力；确认流程稳定后再移除该 Header。
				// "X-MCP-Readonly": "true",
			},
		},
		mcp.WithName("github"),
	)
	defer githubToolSet.Close()
	if err := githubToolSet.Init(ctx); err != nil {
		log.Fatalf("init GitHub MCP toolset: %v", err)
	}

	policy := &tool.RetryPolicy{
		MaxAttempts:     3,                      // 包括第一次,共试 3 次
		InitialInterval: 200 * time.Millisecond, // 第一次失败等多久
		BackoffFactor:   2.0,                    // 每次失败间隔翻倍
	}

	// ============ 6. Agent ============
	llmAgent := llmagent.New(
		"demo-assistant",
		llmagent.WithModel(llm),
		llmagent.WithDescription("一个能记事的天气助手"),
		llmagent.WithInstruction(
			"你叫小天，用户的天气/时间助手。\n"+
				"按需调用工具解决用户问题。\n"+
				"规则：\n"+
				"1. 文件操作严格限制在 workspace 目录内，超出要先问。\n"+
				"2. 查询 GitHub 前先从用户输入确认 owner/repo，不要猜测目标仓库。\n"+
				"3. 简洁回复，不要啰嗦。",
		),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream: true,
		}),
		llmagent.WithTools(tools), // 天气工具 + memory_add/update/search/load
		llmagent.WithToolSets([]tool.ToolSet{
			claudeToolSet,
			tavilyToolSet,
			githubToolSet,
		}),
		llmagent.WithPreloadMemory(10),
		llmagent.WithToolCallRetryPolicy(policy),
	)

	// ============ 7. Runner ============
	r := runner.NewRunner(
		"demo-app",
		llmAgent,
		runner.WithSessionService(sessionService),
		runner.WithMemoryService(memoryService),
	)
	defer r.Close()

	// ============ 8. AG-UI HTTP Server（与命令行并列运行）============
	aguiAddr := envOr("AGUI_ADDR", "127.0.0.1:8080")
	aguiServer, err := agui.New(r,
		agui.WithPath("/agui"),
		agui.WithAppName(sessionAppName),
		agui.WithSessionService(sessionService),          // 让 AG-UI 写 track_events
		agui.WithMessagesSnapshotEnabled(true),           // 注册 /history 端点供 HttpAgent 拉历史
		agui.WithAGUIRunnerOptions(
			aguirunner.WithUserIDResolver(func(ctx context.Context, input *aguiadapter.RunAgentInput) (string, error) {
				return sessionUserID, nil
			}),
		),
	)
	if err != nil {
		log.Fatalf("create agui server: %v", err)
	}
	// 用 mux 聚合: /agui (聊天) + /api/sessions (列表)
	mux := http.NewServeMux()
	mux.Handle("/agui", aguiServer.Handler())
	mux.HandleFunc("/api/sessions", listSessionsHandler(sessionService))

	go func() {
		log.Printf("AG-UI server  : http://%s/agui", aguiAddr)
		log.Printf("Sessions API  : http://%s/api/sessions", aguiAddr)
		if err := http.ListenAndServe(aguiAddr, mux); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	// ============ 8. 交互式多轮对话 ============
	const sessionID = "011" // 命令行的固定会话ID; AG-UI Server 用 threadId (每次新生成)

	fmt.Println("小天已上线，输入内容开始对话（exit / quit / q 退出）")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n👤 你: ")
		if !scanner.Scan() { // Ctrl+D / EOF
			break
		}

		q := strings.TrimSpace(scanner.Text())
		if q == "" {
			continue
		}
		if q == "exit" || q == "quit" || q == "q" {
			break
		}

		if err := chat(ctx, r, sessionUserID, sessionID, q); err != nil {
			fmt.Printf("  ❌ %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("\n⚠️ 输入读取异常: %v\n", err)
	}

	fmt.Println("\n✅ 再见。Session 已持久化到 demo.db，下次用相同 sessionID 可继续。")
}

// chat 跑一次 Run，消费事件流并打印
func chat(ctx context.Context, r runner.Runner, userID, sessionID, text string) error {
	streamMode := true
	eventCh, err := r.Run(ctx, userID, sessionID, model.NewUserMessage(text),
		// 开流式（虽然 Agent 配了 Stream: true，这里再显式一次保险）
		agent.WithStream(streamMode),
	)
	if err != nil {
		return fmt.Errorf("r.Run: %w", err)
	}

	var streamingText bool
	fmt.Print("🤖 Assistant: ")
	for ev := range eventCh {
		if ev.Error != nil {
			fmt.Printf("\n  ❌ [Error] %s\n", ev.Error.Message)
			continue
		}
		if ev.IsRunnerCompletion() {
			break
		}
		if len(ev.Response.Choices) > 0 {
			for _, choice := range ev.Response.Choices {
				// 模型决定调用工具时，打印工具名和 JSON 参数。
				for _, call := range choice.Message.ToolCalls {
					fmt.Printf("\n🔧 Tool call: %s(%s)\n",
						call.Function.Name,
						string(call.Function.Arguments),
					)
				}

				// 工具执行完成后，打印工具实际返回的内容。
				if choice.Message.Role == model.RoleTool {
					fmt.Printf("\n📦 Tool result [%s]: %s\n",
						choice.Message.ToolName,
						choice.Message.Content,
					)
				}

				if content := choice.Delta.Content; content != "" {
					fmt.Print(content)
					streamingText = true
				}
			}
		}
	}
	if streamingText {
		fmt.Println()
	}

	return nil
}

// ============ 工具实现（文件顶级，跟官方示例一致）============

func getWeather(ctx context.Context, args weatherArgs) (weatherResult, error) {
	_ = ctx // 工具签名保留 ctx，方便日后加 ctx 传递的逻辑（如取消、超时、ToolCallID）
	if r, ok := knownCities[args.Location]; ok {
		r.Note = "（真实 Mock 数据）"
		return r, nil
	}
	return weatherResult{
		Location:    args.Location,
		Weather:     conditions[rand.Intn(len(conditions))],
		Temperature: fmt.Sprintf("%d°C", 15+rand.Intn(20)),
		Note:        "（随机 Mock 数据）",
	}, nil
}

func getCurrentTime(ctx context.Context, args currentTimeArgs) (currentTimeResult, error) {
	_ = ctx
	// 优先用 IANA 时区名（"Asia/Shanghai" / "America/New_York"）；空或非法一律回退北京时间。
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

// ============ 工具数据类型 ============

type weatherArgs struct {
	Location string `json:"location" jsonschema:"description=城市名，例如 杭州/北京/上海"`
}

type weatherResult struct {
	Location    string `json:"location"`
	Weather     string `json:"weather"`
	Temperature string `json:"temperature"`
	Note        string `json:"note"`
}

type currentTimeArgs struct {
	// 用 string 而不是 *time.Location：后者生成的 JSON Schema 没法表达，模型容易传错。
	// time.LoadLocation 支持的名字（"Asia/Shanghai"/"America/New_York"），空或非法则回退北京时间。
	Timezone string `json:"timezone" jsonschema:"description=IANA 时区名，例如 Asia/Shanghai 或 America/New_York,留空用北京时间"`
}

type currentTimeResult struct {
	Location    string `json:"location"`
	Time        string `json:"time"`
	Weekday     string `json:"weekday"`
	Timezone    string `json:"timezone"`
	UnixSeconds int64  `json:"unix_seconds"`
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ============ /api/sessions: 列出当前用户的所有会话 ============
type sessionItem struct {
	ID         string    `json:"id"`
	UpdatedAt  time.Time `json:"updated_at"`
	EventCount int       `json:"event_count"`
	Title      string    `json:"title"`
}

func listSessionsHandler(svc session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessions, err := svc.ListSessions(r.Context(), session.UserKey{
			AppName: sessionAppName,
			UserID:  sessionUserID,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := make([]sessionItem, 0, len(sessions))
		for _, s := range sessions {
			items = append(items, sessionItem{
				ID:         s.ID,
				UpdatedAt:  s.UpdatedAt,
				EventCount: len(s.Events),
				Title:      s.ID, // 用 ID 当标题（最简单，可读性还行）
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"sessions": items})
	}
}
