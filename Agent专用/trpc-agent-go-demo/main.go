// trpc-agent-go 最小可运行 Demo
//
// 展示：Runner + LLMAgent + OpenAI 兼容 Model + SQLite Session + Agentic Memory + Mock 工具
//
// 运行：
//
//	export OPENAI_API_KEY="sk-xxx"
//	export OPENAI_BASE_URL="https://api.deepseek.com/v1"   # 可选，默认 OpenAI
//	go run .
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"

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
)

func main() {
	ctx := context.Background()

	// ============ 1. Model（OpenAI 兼容协议，DeepSeek/GPT 都用这个） ============
	modelName := envOr("MODEL_NAME", "deepseek-chat")
	llm := openai.New(modelName)

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
	weatherTool := newWeatherTool()
	tools := append([]tool.Tool{weatherTool}, memoryService.Tools()...)

	// ============ 6. Agent ============
	llmAgent := llmagent.New(
		"demo-assistant",
		llmagent.WithModel(llm),
		llmagent.WithDescription("一个能记事的天气助手"),
		llmagent.WithInstruction(
			"你叫小天，能查天气，也能记住用户告诉你的重要信息。\n"+
				"规则：\n"+
				"1. 用户提到个人信息（姓名/偏好/常住地/职业）时，主动用 memory 工具存起来。\n"+
				"2. 回答前如果想确认某事，先用 memory 工具查一下。\n"+
				"3. 简洁回复，不要啰嗦。",
		),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream: true,
		}),
		llmagent.WithTools(tools), // 天气工具 + memory_add/update/search/load
	)

	// ============ 7. Runner ============
	r := runner.NewRunner(
		"demo-app",
		llmAgent,
		runner.WithSessionService(sessionService),
		runner.WithMemoryService(memoryService),
	)
	defer r.Close()

	// ============ 8. 多轮对话 ============
	const userID = "u-alice"
	const sessionID = "s-demo-002"

	dialogues := []string{
		"查询下深圳天气😀",
	}

	for i, q := range dialogues {
		fmt.Printf("\n━━━━━ Round %d ━━━━━\n", i+1)
		fmt.Printf("👤 Alice: %s\n", q)

		if err := chat(ctx, r, userID, sessionID, q); err != nil {
			log.Fatalf("round %d failed: %v", i+1, err)
		}
	}

	fmt.Println("\n✅ Demo 完成。Session 已持久化到 demo.db，重启后用相同 sessionID 还能继续。")
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
			choice := ev.Response.Choices[0]

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
	if streamingText {
		fmt.Println()
	}

	return nil
}

// ============ 天气 Tool（Mock）============

type weatherInput struct {
	Location string `json:"location" jsonschema:"description=城市名，例如 杭州/北京/上海"`
}

type weatherOutput struct {
	Location    string `json:"location"`
	Weather     string `json:"weather"`
	Temperature string `json:"temperature"`
	Note        string `json:"note"`
}

func newWeatherTool() tool.Tool {
	// 固定几个城市的数据，其他城市随机生成
	known := map[string]weatherOutput{
		"杭州": {Location: "杭州", Weather: "多云转晴", Temperature: "26°C"},
		"北京": {Location: "北京", Weather: "晴", Temperature: "30°C"},
		"上海": {Location: "上海", Weather: "小雨", Temperature: "24°C"},
		"深圳": {Location: "深圳", Weather: "雷阵雨", Temperature: "31°C"},
	}

	conditions := []string{"晴", "多云", "阴", "小雨", "中雨", "雷阵雨"}

	return function.NewFunctionTool(
		func(ctx context.Context, in weatherInput) (weatherOutput, error) {
			if out, ok := known[in.Location]; ok {
				out.Note = "（真实 Mock 数据）"
				return out, nil
			}

			// 未知城市：随机生成
			return weatherOutput{
				Location:    in.Location,
				Weather:     conditions[rand.Intn(len(conditions))],
				Temperature: fmt.Sprintf("%d°C", 15+rand.Intn(20)),
				Note:        "（随机 Mock 数据）",
			}, nil
		},
		function.WithName("get_weather"),
		function.WithDescription("查询指定城市的天气（Mock 数据，仅用于 Demo）"),
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
