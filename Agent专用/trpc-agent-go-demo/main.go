// trpc-agent-go 最小可运行 Demo
//
// 展示：Runner + LLMAgent + OpenAI 兼容 Model + SQLite Session + Agentic Memory + Mock 工具
//
// 运行：
//
//	export OPENAI_API_KEY="sk-xxx"
//	export OPENAI_BASE_URL="https://api.minimaxi.com/v1"   # MiniMax M3 OpenAI 兼容端点
//	export MODEL_NAME="MiniMax-M3"
//	go run .
//
// ════════════════════════════════════════════════════════════════════
// ❶# NOTO 【M3 reasoning_split 坑】
// ────────────────────────────────────────────────────────────────────
// MiniMax-M3 默认走 OpenAI "原生格式"：把 thinking 嵌进 content 字段
// 用  ̶t̶h̶i̶n̶k̶ ... ̶/̶t̶h̶i̶n̶k̶  包裹（污染正文）。
// 必须显式加 `reasoning_split: true`，M3 才会把 thinking 单独放到
// reasoning_details 字段，框架的 messageCollector 才能正确分离。
// 不加 → 流式输出会看到一堆  ̶t̶h̶i̶n̶k̶  标签被当作文本打印。
//
// 调试方法：开打  https://api.minimaxi.com/v1/chat/completions 看
// 第一轮响应里 reasoning_details 字段有没有单独出现。
// ════════════════════════════════════════════════════════════════════
package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	memorysqlite "trpc.group/trpc-go/trpc-agent-go/memory/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	agenttool "trpc.group/trpc-go/trpc-agent-go/tool/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool/claudecode"
	"trpc.group/trpc-go/trpc-agent-go/tool/file"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// Tavily API key（硬编码演示；生产请走环境变量或 secret store）。
const tavilyAPIKey = "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"

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
			ServerURL: "https://mcp.tavily.com/mcp/?tavilyApiKey=" + tavilyAPIKey,
			Timeout:   30 * time.Second,
		},
		mcp.WithToolFilterFunc(tool.NewIncludeToolNamesFilter("tavily_search", "tavily_extract")),
	)
	defer tavilyToolSet.Close()
	// Init 提前发现连接失败(否则要等模型实际调用才报错)
	if err := tavilyToolSet.Init(ctx); err != nil {
		log.Fatalf("init tavily toolset: %v", err)
	}

	// AgentTool:包一个"作家专家"子 Agent,工具用 tool/file 包(对相对路径友好)。
	writerFileToolSet, err := file.NewToolSet(
		file.WithBaseDir("/Users/lucy/Documents/EDITH/workspace"),
	)
	if err != nil {
		log.Fatalf("new file toolset: %v", err)
	}
	defer writerFileToolSet.Close()

	writerAgent := newWriterAgent(writerFileToolSet, llm)
	writerTool := agenttool.NewTool(
		writerAgent,
		agenttool.WithSkipSummarization(false),
		agenttool.WithStreamInner(false),
		agenttool.WithInnerTextMode(agenttool.InnerTextModeExclude),
		agenttool.WithResponseMode(agenttool.ResponseModeFinalOnly),
	)
	tools = append(tools, writerTool)

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
				"2. 简洁回复，不要啰嗦。",
		),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream: true,
		}),
		llmagent.WithTools(tools), // 天气工具 + memory_add/update/search/load
		llmagent.WithToolSets([]tool.ToolSet{claudeToolSet, tavilyToolSet}), // claudecode + tavily_search / tavily_extract
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

	// ============ 8. 交互式多轮对话 ============
	const userID = "u-alice"
	const sessionID = "009"

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

		if err := chat(ctx, r, userID, sessionID, q); err != nil {
			fmt.Printf("  ❌ %v\n", err)
		}
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

// newWriterAgent 包一个"作家专家"子 Agent,工具交给 tool/file 包的文件 ToolSet。
// 对比 claudecode:tool/file 默认把相对路径 join 到 baseDir,LLM 不用硬记绝对路径。
func newWriterAgent(fileSet tool.ToolSet, llm model.Model) agent.Agent {
	return llmagent.New(
		"writer-specialist",
		llmagent.WithModel(llm),
		llmagent.WithDescription("把写作任务外包给写作专家。它能读/写 workspace 下的文件,没有 Bash/Grep 这些副作用强的工具。"),
		llmagent.WithInstruction(
			"你是写作专家子 Agent。父 Agent 会给 'task'(要写什么) 和 'file_path'(写到哪)。\n"+
				"file_path 可以传相对路径(如 story.md),不需要写绝对路径。\n"+
				"先用读工具看上下文(若有),再用写工具落盘。完成后只输出:'已完成:[一句话概括写进去什么]'。",
		),
		llmagent.WithToolSets([]tool.ToolSet{fileSet}),
	)
}

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
