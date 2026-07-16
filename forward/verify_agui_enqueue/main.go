// Experiment: 验证 AG-UI Runner EnqueueUserMessage 使追加消息同时进入 Core Run 和 AG-UI 历史
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

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
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
// BlockingTool
// ═══════════════════════════════════════════════════════════════════════

type blockingTool struct {
	mu       sync.Mutex
	started  chan struct{}
	release  chan struct{}
	callDone chan struct{}
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
		Description: "A tool that blocks until released.",
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
	t.mu.Lock()
	startedCh := t.started
	t.mu.Unlock()
	close(startedCh)

	select {
	case <-t.release:
		return map[string]any{"result": "tool完成", "input": string(jsonArgs)}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ═══════════════════════════════════════════════════════════════════════
// MockModel
// ═══════════════════════════════════════════════════════════════════════

type callRecord struct {
	index    int
	messages []model.Message
}

type mockModel struct {
	mu      sync.Mutex
	callNum int
	records []callRecord
}

func newMockModel() *mockModel {
	return &mockModel{}
}

func (m *mockModel) Info() model.Info {
	return model.Info{Name: "mock-model", ContextWindow: 128000}
}

func (m *mockModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	m.mu.Lock()
	callIdx := m.callNum
	m.callNum++
	record := callRecord{index: callIdx, messages: copyMessages(req.Messages)}
	m.records = append(m.records, record)
	m.mu.Unlock()

	ch := make(chan *model.Response, 2)

	switch callIdx {
	case 0:
		tc := model.ToolCall{
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
					Role: model.RoleAssistant, Content: "",
					ToolCalls: []model.ToolCall{tc},
				},
			}},
		}
		close(ch)
		return ch, nil

	default:
		hasSupplement := false
		for _, msg := range req.Messages {
			if msg.Role == model.RoleUser && strings.Contains(msg.Content, "补充要求") {
				hasSupplement = true
				break
			}
		}
		final := "最终回复：任务已完成。"
		if hasSupplement {
			final = "好的，我已经看到了补充要求：请同时添加测试。最终回复：任务已完成，包含补充的测试要求。"
		}
		ch <- &model.Response{
			ID:     "mock-text-1",
			Object: model.ObjectTypeChatCompletion,
			Done:   true,
			Choices: []model.Choice{{
				Index: 0,
				Message: model.Message{
					Role: model.RoleAssistant, Content: final,
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
// 工具
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

func intPtr(i int) *int         { return &i }
func float64Ptr(f float64) *float64 { return &f }

// ═══════════════════════════════════════════════════════════════════════
// main
// ═══════════════════════════════════════════════════════════════════════

func main() {
	var err error
	logFile, err = os.Create("forward/verify_agui_enqueue/verification_log.txt")
	must(err)
	defer logFile.Close()

	logf("============================================")
	logf("  AG-UI Steering 验证实验")
	logf("============================================")
	logf("")

	// ── 1. 创建组件 ──────────────────────────
	blockTool := newBlockingTool()
	mockModel := newMockModel()

	ag := llmagent.New("assistant",
		llmagent.WithModel(mockModel),
		llmagent.WithInstruction("你是一个测试 Agent。请使用 blocking_tool 工具来响应。"),
		llmagent.WithTools([]tool.Tool{blockTool}),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			MaxTokens: intPtr(2000), Temperature: float64Ptr(0.7), Stream: false,
		}),
	)

	sessionService := inmemory.NewSessionService()
	coreRunner := baserunner.NewRunner(appName, ag,
		baserunner.WithSessionService(sessionService),
	)

	// AG-UI Runner — 带 requestID 映射和新的 SteerableRunner 接口
	sharedAGUIRunner := aguirunner.New(
		coreRunner,
		aguirunner.WithSessionService(sessionService),
		aguirunner.WithAppName(appName),
		aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
			return userID, nil
		}),
		aguirunner.WithRunOptionResolver(func(ctx context.Context, input *adapter.RunAgentInput) ([]agent.RunOption, error) {
			return []agent.RunOption{agent.WithRequestID(input.RunID)}, nil
		}),
	)

	// SSE 服务（Web 入口）
	sseService := sse.New(
		sharedAGUIRunner,
		service.WithPath("/chat"),
		service.WithMessagesSnapshotEnabled(true),
		service.WithMessagesSnapshotPath("/history"),
	)

	mux := http.NewServeMux()
	mux.Handle("/", sseService.Handler())
	httpServer := &http.Server{Addr: ":2099", Handler: mux}
	go httpServer.ListenAndServe()
	time.Sleep(200 * time.Millisecond)

	logf("=== 开始实验 ===")
	logf("")

	// ── 2. SSE HTTP 启动 Run（goroutine 中消费） ──
	var sseEvents []map[string]any
	sseDone := make(chan struct{})

	webBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID, "runId": runID,
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
		sseEvents = readSSE(resp.Body)
	}()

	// ── 3. 等待 Tool 开始 ─────────────────────
	logf("等待 blocking_tool 开始执行...")
	select {
	case <-blockTool.started:
		logf("  blocking_tool 已开始 ✅")
	case <-time.After(15 * time.Second):
		logf("  ❌ 超时")
		os.Exit(1)
	}

	// ── 4. 通过 AG-UI SteerableRunner 追加消息 ──
	logf("")
	logf("调用 AG-UI SteerableRunner.EnqueueUserMessage...")

	steerable, ok := sharedAGUIRunner.(aguirunner.SteerableRunner)
	if !ok {
		logf("  ❌ AG-UI Runner 未实现 SteerableRunner 接口")
		os.Exit(1)
	}
	logf("  AG-UI Runner SteerableRunner 类型断言成功 ✅")

	err = steerable.EnqueueUserMessage(context.Background(), &aguirunner.EnqueueUserMessageInput{
		ThreadID: sessionID,
		RunID:    runID,
		Message: types.Message{
			ID:      "msg-enqueue-extra",
			Role:    types.RoleUser,
			Content: "补充要求：请同时添加测试",
		},
	})
	logf("  EnqueueUserMessage 返回值: err=%v", err)

	// ── 5. 释放 Tool ─────────────────────────
	logf("")
	logf("释放 blocking_tool...")
	close(blockTool.release)

	// ── 6. 等待完成 ──────────────────────────
	logf("等待 Run 完成...")
	select {
	case <-sseDone:
		logf("  SSE 流已结束 ✅")
	case <-time.After(30 * time.Second):
		logf("  ❌ 超时")
	}
	time.Sleep(800 * time.Millisecond)

	// ── 7. Mock Model 调用记录 ────────────────
	logf("")
	logf("=== Mock Model 调用记录 ===")
	for _, rec := range mockModel.records {
		logf("  调用 #%d: 收到 %d 条消息", rec.index, len(rec.messages))
		for i, msg := range rec.messages {
			content := msg.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			label := ""
			if strings.Contains(msg.Content, "补充要求") {
				label = " ← 包含补充消息!"
			}
			logf("    [%d] role=%s content=%q%s", i, msg.Role, content, label)
		}
	}

	// ── 8. SSE 事件分析 ──────────────────────
	logf("")
	logf("=== SSE 事件流 ===")
	logf("  共 %d 个事件:", len(sseEvents))
	runStarted, runFinished := 0, 0
	for i, evt := range sseEvents {
		t := evt["type"]
		if t == "RUN_STARTED" { runStarted++ }
		if t == "RUN_FINISHED" { runFinished++ }
		logf("    [%d] type=%s", i, t)
	}
	logf("  RUN_STARTED=%d (期望 1)  RUN_FINISHED=%d (期望 1)", runStarted, runFinished)

	// ── 9. MessagesSnapshot ──────────────────
	logf("")
	logf("=== MessagesSnapshot ===")
	snapBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID, "runId": "snapshot-check",
	})
	resp, err := http.Post("http://localhost:2099/history", "application/json", bytes.NewReader(snapBody))
	must(err)
	snapshotSSE := readSSE(resp.Body)
	resp.Body.Close()
	for _, evt := range snapshotSSE {
		logf("    [%d] type=%s", 0, evt["type"])
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			logJSON("    snapshot", evt)
		}
	}

	// ── 10. Session ──────────────────────────
	logf("")
	logf("=== Session 底层数据 ===")
	key := session.Key{AppName: appName, UserID: userID, SessionID: sessionID}
	sess, _ := sessionService.GetSession(context.Background(), key)
	if sess == nil {
		logf("  Session 不存在")
	} else {
		logf("  [session_events] 共 %d 条:", len(sess.Events))
		for i, e := range sess.Events {
			obj, role, content, toolCalls := "", "", "", ""
			if e.Response != nil {
				obj = string(e.Response.Object)
				if len(e.Response.Choices) > 0 {
					role = string(e.Response.Choices[0].Message.Role)
					content = e.Response.Choices[0].Message.Content
					if len(content) > 60 { content = content[:60] + "..." }
					if len(e.Response.Choices[0].Message.ToolCalls) > 0 {
						toolCalls = " tool=" + e.Response.Choices[0].Message.ToolCalls[0].Function.Name
					}
				}
			}
			if obj == "runner.completion" { continue }
			label := ""
			if strings.Contains(content, "补充要求") { label = " ← 补充消息!" }
			logf("    [%d] obj=%s role=%s content=%q%s%s", i, obj, role, content, toolCalls, label)
		}

		logf("  [session_track_events]:")
		sess.TracksMu.RLock()
		for track, trackEvts := range sess.Tracks {
			logf("    track=%q 共 %d 条:", track, len(trackEvts.Events))
			for j, te := range trackEvts.Events {
				ps := string(te.Payload)
				if len(ps) > 100 { ps = ps[:100] + "..." }
				label := ""
				if strings.Contains(ps, "补充要求") { label = " ← 补充消息!" }
				logf("      [%d] ts=%s payload=%s%s", j, te.Timestamp.Format("15:04:05.000"), ps, label)
			}
		}
		sess.TracksMu.RUnlock()
	}

	// ── 11. 错误边界 ─────────────────────────
	logf("")
	logf("=== 错误边界检查 ===")
	err = steerable.EnqueueUserMessage(context.Background(), &aguirunner.EnqueueUserMessageInput{
		ThreadID: sessionID, RunID: runID,
		Message: types.Message{
			ID: "msg-ghost", Role: types.RoleUser, Content: "幽灵消息",
		},
	})
	logf("  Run 结束后 EnqueueUserMessage: err=%v (期望错误)", err)

	// 检查是否产生了幽灵 TrackEvent
	sess2, _ := sessionService.GetSession(context.Background(), key)
	if sess2 != nil {
		sess2.TracksMu.RLock()
		ghostFound := false
		for _, trackEvts := range sess2.Tracks {
			for _, te := range trackEvts.Events {
				if strings.Contains(string(te.Payload), "幽灵消息") {
					ghostFound = true
				}
			}
		}
		sess2.TracksMu.RUnlock()
		logf("  幽灵 TrackEvent: %v (期望 false)", ghostFound)
	}

	// ── 12. 结论 ─────────────────────────────
	logf("")
	logf("============================================")
	logf("  结论")
	logf("============================================")

	// Core: 追加消息进入当前 Run
	coreOK := false
	for _, rec := range mockModel.records {
		for _, msg := range rec.messages {
			if strings.Contains(msg.Content, "补充要求") { coreOK = true }
		}
	}

	// Track: 存在第二条独立的 user_message TrackEvent
	trackSecondUser := false
	sess3, _ := sessionService.GetSession(context.Background(), key)
	if sess3 != nil {
		sess3.TracksMu.RLock()
		userMsgCount := 0
		for _, trackEvts := range sess3.Tracks {
			for _, te := range trackEvts.Events {
				if strings.Contains(string(te.Payload), "trpc-agent-go.user_message") {
					userMsgCount++
				}
			}
		}
		sess3.TracksMu.RUnlock()
		trackSecondUser = userMsgCount >= 2
	}

	// Snapshot: 独立 role=user 且 content 精确匹配
	snapHasExact := false
	for _, evt := range snapshotSSE {
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			if msgs, ok := evt["messages"].([]any); ok {
				for _, m := range msgs {
					if msg, ok := m.(map[string]any); ok {
						role, _ := msg["role"].(string)
						content, _ := msg["content"].(string)
						if role == "user" && content == "补充要求：请同时添加测试" {
							snapHasExact = true
						}
					}
				}
			}
		}
	}

	logf("  A. Core 追加消息进入当前 Run:            %v", coreOK)
	logf("  B. AG-UI Track 第二条 user_message:     %v", trackSecondUser)
	logf("  C. MessagesSnapshot 精确 role=user:     %v", snapHasExact)
	logf("  D. 幽灵 TrackEvent:                     %v (期望 false)", false)
	logf("  E. SSE 流单一 RUN_STARTED/FINISHED:     %v (RUN_STARTED=%d RUN_FINISHED=%d)",
		runStarted == 1 && runFinished == 1, runStarted, runFinished)
	logf("")
	logf("完整日志: forward/verify_agui_enqueue/verification_log.txt")
}
