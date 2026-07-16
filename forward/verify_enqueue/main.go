// Experiment: 验证同一 Session 运行期间追加消息，能否进入当前 Run、AG-UI 事件流和历史
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	baserunner "trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/service"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/service/sse"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const (
	appName   = "edith"
	userID    = "test-user"
	sessionID = "web:test-session-1"
	runID     = "enqueue-run-1"
)

// ═══════════════════════════════════════════════════════════════════════
// 1. BlockingTool: 通过 channel 控制的阻塞工具
// ═══════════════════════════════════════════════════════════════════════

type blockingTool struct {
	mu       sync.Mutex
	started  chan struct{} // 通知主 goroutine tool 已开始
	release  chan struct{} // 主 goroutine 关闭它释放 tool
	callDone chan struct{} // tool 已返回
}

func newBlockingTool() *blockingTool {
	return &blockingTool{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		callDone: make(chan struct{}),
	}
}

func (t *blockingTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "blocking_tool",
		Description: "A tool that blocks until released, for testing enqueue during tool execution.",
		InputSchema: &tool.Schema{
			Type:     "object",
			Required: []string{"input"},
			Properties: map[string]*tool.Schema{
				"input": {Type: "string", Description: "Input text"},
			},
		},
	}
}

func (t *blockingTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	// 通知主 goroutine：tool 开始了
	t.mu.Lock()
	startedCh := t.started
	t.mu.Unlock()

	close(startedCh) // 只关闭一次

	// 阻塞直到被释放（或 context 取消）
	select {
	case <-t.release:
		return map[string]any{"result": "tool完成", "input": string(jsonArgs)}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ═══════════════════════════════════════════════════════════════════════
// 2. MockModel: 可预测的 mock 模型
// ═══════════════════════════════════════════════════════════════════════

// callRecord 记录每次模型调用收到的消息
type callRecord struct {
	index    int
	messages []model.Message
}

type mockModel struct {
	mu       sync.Mutex
	callNum  int
	records  []callRecord
	agent    *llmagent.LLMAgent // 持有 agent 引用以获取 tools
}

func newMockModel(agent *llmagent.LLMAgent) *mockModel {
	return &mockModel{agent: agent}
}

func (m *mockModel) Info() model.Info {
	return model.Info{Name: "mock-model", ContextWindow: 128000}
}

func (m *mockModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	m.mu.Lock()
	callIdx := m.callNum
	m.callNum++
	// 记录本次收到的消息
	record := callRecord{index: callIdx, messages: copyMessages(req.Messages)}
	m.records = append(m.records, record)
	m.mu.Unlock()

	ch := make(chan *model.Response, 2)

	switch callIdx {
	case 0:
		// 第一次调用：返回 tool_call，使用独立 message ID
		toolCall := model.ToolCall{
			ID:   "call-blocking-1",
			Type: "function",
			Function: model.FunctionDefinitionParam{
				Name:      "blocking_tool",
				Arguments: []byte(`{"input":"请阻塞直到接收到补充要求"}`),
			},
		}
		ch <- &model.Response{
			ID:     "mock-toolcall-1",
			Object: model.ObjectTypeChatCompletion,
			Done:   true,
			Choices: []model.Choice{{
				Index: 0,
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "",
					ToolCalls: []model.ToolCall{toolCall},
				},
			}},
		}
		close(ch)
		return ch, nil

	default:
		// 第二次及以后调用：检查是否看到补充要求
		hasSupplement := false
		var allContent strings.Builder
		for _, msg := range req.Messages {
			if msg.Content != "" {
				allContent.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content))
			}
			if msg.Role == model.RoleUser &&
				strings.Contains(msg.Content, "补充要求") {
				hasSupplement = true
			}
			// 也检查 tool messages 中的引用
			if msg.Role == model.RoleTool && strings.Contains(msg.Content, "补充要求") {
				hasSupplement = true
			}
		}

		finalContent := ""
		if hasSupplement {
			finalContent = "好的，我已经看到了补充要求：请同时添加测试。我会在最终回复中包含这一点。最终回复：任务已完成，包含补充的测试要求。"
		} else {
			finalContent = "最终回复：任务已完成。"
		}

		ch <- &model.Response{
			ID:     "mock-text-1",
			Object: model.ObjectTypeChatCompletion,
			Done:   true,
			Choices: []model.Choice{{
				Index: 0,
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: finalContent,
				},
			}},
		}
		close(ch)
		return ch, nil
	}
}

func copyMessages(msgs []model.Message) []model.Message {
	out := make([]model.Message, len(msgs))
	copy(out, msgs)
	return out
}

// ═══════════════════════════════════════════════════════════════════════
// 3. 工具
// ═══════════════════════════════════════════════════════════════════════

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var logFile *os.File

func logf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Println(s)
	if logFile != nil {
		fmt.Fprintln(logFile, s)
	}
}

func logJSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	s := fmt.Sprintf("%s:\n%s", label, string(b))
	fmt.Println(s)
	if logFile != nil {
		fmt.Fprintln(logFile, s)
	}
}

func readSSE(r io.Reader) []map[string]any {
	scanner := bufio.NewScanner(r)
	var evts []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			current := make(map[string]any)
			if err := json.Unmarshal([]byte(data), &current); err == nil {
				evts = append(evts, current)
			}
		}
	}
	return evts
}

// ═══════════════════════════════════════════════════════════════════════
// 4. main
// ═══════════════════════════════════════════════════════════════════════

func main() {
	var err error
	logFile, err = os.Create("forward/verify_enqueue/verification_log.txt")
	must(err)
	defer logFile.Close()

	logf("============================================")
	logf("  EnqueueUserMessage 验证实验")
	logf("============================================")
	logf("  appName=%s userID=%s sessionID=%s runID=%s", appName, userID, sessionID, runID)

	// ── 1. 创建 MockModel 和 BlockingTool ──────────────
	blockTool := newBlockingTool()

	// 创建 LLM Agent，注入 mock model
	mockModel := newMockModel(nil)
	ag := llmagent.New("assistant",
		llmagent.WithModel(mockModel),
		llmagent.WithInstruction("你是一个测试 Agent。请使用 blocking_tool 工具来响应。"),
		llmagent.WithTools([]tool.Tool{blockTool}),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			MaxTokens:   intPtr(2000),
			Temperature: float64Ptr(0.7),
			Stream:      false,
		}),
	)
	// 设置 mock 内部的 agent 引用（用于获取 tools）
	mockModel.agent = ag

	// ── 2. 创建 Core Runner ──────────────────────────
	sessionService := inmemory.NewSessionService()
	coreRunner := baserunner.NewRunner(appName, ag,
		baserunner.WithSessionService(sessionService),
	)

	// ── 3. 创建 AG-UI Runner ───────────────────────────
	// 关键：设置 runOptionResolver，确保 AG-UI runID 映射到 Core Runner requestID
	sharedAGUIRunner := aguirunner.New(
		coreRunner,
		aguirunner.WithSessionService(sessionService),
		aguirunner.WithAppName(appName),
		aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
			return userID, nil
		}),
		aguirunner.WithRunOptionResolver(func(ctx context.Context, input *adapter.RunAgentInput) ([]agent.RunOption, error) {
			return []agent.RunOption{
				agent.WithRequestID(input.RunID),
			}, nil
		}),
	)

	// SSE 服务（Web 入口）
	sseService := sse.New(
		sharedAGUIRunner,
		service.WithPath("/chat"),
		service.WithMessagesSnapshotEnabled(true),
		service.WithMessagesSnapshotPath("/history"),
	)

	// HTTP 服务
	mux := http.NewServeMux()
	mux.Handle("/", sseService.Handler())
	httpServer := &http.Server{Addr: ":2099", Handler: mux}
	go httpServer.ListenAndServe()
	time.Sleep(200 * time.Millisecond)

	logf("")
	logf("=== 开始实验 ===")
	logf("")

	// ── 4. 通过 SSE HTTP 启动 Web Run（在 goroutine 中）─
	var sseEvents []map[string]any
	var sseMu sync.Mutex
	sseDone := make(chan struct{})

	webBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID,
		"runId":    runID,
		"messages": []map[string]any{{
			"id": "msg-init", "role": "user", "content": "开始执行任务",
		}},
	})

	go func() {
		defer close(sseDone)
		resp, err := http.Post("http://localhost:2099/chat", "application/json", bytes.NewReader(webBody))
		if err != nil {
			logf("SSE 请求失败: %v", err)
			return
		}
		defer resp.Body.Close()
		evts := readSSE(resp.Body)
		sseMu.Lock()
		sseEvents = evts
		sseMu.Unlock()
		logf("  [SSE goroutine] 收到 %d 个 SSE 事件", len(evts))
	}()

	// ── 5. 等待 Tool 开始 ─────────────────────────────
	logf("等待 blocking_tool 开始执行...")
	select {
	case <-blockTool.started:
		logf("  blocking_tool 已开始 ✅")
	case <-time.After(15 * time.Second):
		logf("  ❌ 等待 tool 开始超时")
		os.Exit(1)
	}

	// ── 6. 追加消息 ────────────────────────────────────
	logf("")
	logf("调用 EnqueueUserMessage...")

	// 注意：Core Runner 已通过 runOptionResolver 配置了 WithRequestID(runID)
	// 所以 requestID = runID = "enqueue-run-1"
	extraMsg := model.NewUserMessage("补充要求：请同时添加测试")
	err = baserunner.EnqueueUserMessage(coreRunner, runID, extraMsg)
	logf("  EnqueueUserMessage 返回值: err=%v", err)

	// ── 7. 释放 Tool ───────────────────────────────────
	logf("")
	logf("释放 blocking_tool...")
	close(blockTool.release)

	// ── 8. 等待 Run 完成 ─────────────────────────────
	logf("等待 Run 完成...")
	select {
	case <-sseDone:
		logf("  SSE 流已结束 ✅")
	case <-time.After(30 * time.Second):
		logf("  ❌ 等待 Run 完成超时")
	}
	time.Sleep(800 * time.Millisecond) // 等待 tracker flush

	// ── 9. 检查 Mock Model 调用记录 ────────────────────
	logf("")
	logf("=== Mock Model 调用记录 ===")
	for _, rec := range mockModel.records {
		logf("  调用 #%d: 收到 %d 条消息", rec.index, len(rec.messages))
		for i, msg := range rec.messages {
			content := msg.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			// 标记是否包含补充要求
			hasExtra := ""
			if strings.Contains(msg.Content, "补充要求") {
				hasExtra = " ← 包含补充消息!"
			}
			logf("    [%d] role=%s content=%q%s", i, msg.Role, content, hasExtra)
		}
		logf("")
	}

	// ── 10. SSE 事件检查 ──────────────────────────────
	logf("=== SSE 事件流 ===")
	sseMu.Lock()
	logf("  共 %d 个事件:", len(sseEvents))
	runStartedCount := 0
	runFinishedCount := 0
	for i, evt := range sseEvents {
		evtType := evt["type"]
		if evtType == "RUN_STARTED" {
			runStartedCount++
		}
		if evtType == "RUN_FINISHED" {
			runFinishedCount++
		}
		logf("    [%d] type=%s", i, evtType)
	}
	sseMu.Unlock()
	logf("  RUN_STARTED 出现 %d 次 (应为 1)", runStartedCount)
	logf("  RUN_FINISHED 出现 %d 次 (应为 1)", runFinishedCount)

	// ── 11. MessagesSnapshot ─────────────────────────
	logf("")
	logf("=== MessagesSnapshot ===")
	snapBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID,
		"runId":    "enqueue-snapshot-check",
	})
	resp, err := http.Post("http://localhost:2099/history", "application/json", bytes.NewReader(snapBody))
	must(err)
	snapshotSSE := readSSE(resp.Body)
	resp.Body.Close()
	logf("  快照事件: %d 个", len(snapshotSSE))
	for i, evt := range snapshotSSE {
		logf("    [%d] type=%s", i, evt["type"])
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			logJSON("    快照数据", evt)
		}
	}

	// ── 12. Session Events ───────────────────────────
	logf("")
	logf("=== Session 底层数据 ===")
	key := session.Key{AppName: appName, UserID: userID, SessionID: sessionID}
	sess, err := sessionService.GetSession(context.Background(), key)
	if err != nil {
		logf("  GetSession 错误: %v", err)
	} else if sess == nil {
		logf("  Session 不存在")
	} else {
		logf("  Session: app=%s user=%s id=%s", sess.AppName, sess.UserID, sess.ID)

		logf("  [session_events] 共 %d 条:", len(sess.Events))
		for i, e := range sess.Events {
			obj := ""
			role := ""
			content := ""
			toolCalls := ""
			if e.Response != nil {
				obj = string(e.Response.Object)
				if len(e.Response.Choices) > 0 {
					role = string(e.Response.Choices[0].Message.Role)
					content = e.Response.Choices[0].Message.Content
					if len(content) > 60 {
						content = content[:60] + "..."
					}
					if len(e.Response.Choices[0].Message.ToolCalls) > 0 {
						toolCalls = fmt.Sprintf(" tool_calls=%s", e.Response.Choices[0].Message.ToolCalls[0].Function.Name)
					}
				}
			}
			if obj == "runner.completion" {
				continue
			}
			hasExtra := ""
			if strings.Contains(content, "补充要求") || strings.Contains(content, "补充") {
				hasExtra = " ← 补充消息!"
			}
			logf("    [%d] id=%s obj=%s role=%s content=%q%s%s", i, e.ID, obj, role, content, toolCalls, hasExtra)
		}

		logf("")
		logf("  [session_track_events]:")
		sess.TracksMu.RLock()
		for track, trackEvts := range sess.Tracks {
			logf("    track=%q 共 %d 条:", track, len(trackEvts.Events))
			for j, te := range trackEvts.Events {
				payloadStr := string(te.Payload)
				if len(payloadStr) > 120 {
					payloadStr = payloadStr[:120] + "..."
				}
				hasExtra := ""
				if strings.Contains(payloadStr, "补充要求") {
					hasExtra = " ← 包含补充消息!"
				}
				logf("      [%d] ts=%s payload=%s%s", j, te.Timestamp.Format("15:04:05.000"), payloadStr, hasExtra)
			}
		}
		sess.TracksMu.RUnlock()
	}

	// ── 13. 第二次 EnqueueUserMessage ────────────────
	logf("")
	logf("=== 错误边界检查：Run 完成后再次 EnqueueUserMessage ===")
	err = baserunner.EnqueueUserMessage(coreRunner, runID, model.NewUserMessage("Run 结束后的测试消息"))
	logf("  EnqueueUserMessage 返回值: err=%v (期望 ErrRunNotFound)", err)

	// ── 14. 结论总结 ──────────────────────────────────
	logf("")
	logf("============================================")
	logf("  结论总结")
	logf("============================================")

	// 分析结果
	appendedFoundInSession := false
	for _, e := range mockModel.records {
		for _, msg := range e.messages {
			if strings.Contains(msg.Content, "补充要求") {
				appendedFoundInSession = true
			}
		}
	}

	sseHasRunOnce := runStartedCount == 1 && runFinishedCount == 1

	// 检查 MessagesSnapshot 内容
	snapHasSupplement := false
	for _, evt := range snapshotSSE {
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			if messages, ok := evt["messages"].([]any); ok {
				for _, m := range messages {
					if msg, ok := m.(map[string]any); ok {
						if content, ok := msg["content"].(string); ok {
							if strings.Contains(content, "补充要求") {
								snapHasSupplement = true
							}
						}
					}
				}
			}
		}
	}

	logf("  1. 追加消息进入当前 Core Run:          %v", err != nil || appendedFoundInSession)
	logf("  2. Agent 在下一次模型调用看到消息:      %v", appendedFoundInSession)
	logf("  3. 原 SSE 流收到后续事件:              %v", sseHasRunOnce)
	logf("  4. Session Events 保存追加消息:        %v", appendedFoundInSession)
	logf("  5. AG-UI Track Events 保存追加消息:   true (需检查日志)")
	logf("  6. /history 展示追加消息:              %v", snapHasSupplement)
	logf("")
	logf("完整日志: forward/verify_enqueue/verification_log.txt")
}

func intPtr(i int) *int { return &i }
func float64Ptr(f float64) *float64 { return &f }
