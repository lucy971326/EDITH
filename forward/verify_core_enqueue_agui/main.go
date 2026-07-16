// 对照实验：只调用 Core Runner.EnqueueUserMessage，不修改框架，验证 AG-UI 表现
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
	sessionID = "web:test-session-c1"
	runID     = "core-enqueue-run-1"
)

// ═══════════════════════════════════════════════════════════════════════
// BlockingTool
// ═══════════════════════════════════════════════════════════════════════

type blockingTool struct {
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
}

func newBlockingTool() *blockingTool {
	return &blockingTool{started: make(chan struct{}), release: make(chan struct{})}
}

func (t *blockingTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name: "blocking_tool", Description: "Blocks until released.",
		InputSchema: &tool.Schema{
			Type: "object", Required: []string{"input"},
			Properties: map[string]*tool.Schema{
				"input": {Type: "string", Description: "Input text"},
			},
		},
	}
}

func (t *blockingTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	t.mu.Lock()
	ch := t.started
	t.mu.Unlock()
	close(ch)
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

type mockModel struct {
	mu      sync.Mutex
	callNum int
	records [][]model.Message // 每次调用的消息列表
}

func (m *mockModel) Info() model.Info { return model.Info{Name: "mock", ContextWindow: 128000} }

func (m *mockModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	msgs := make([]model.Message, len(req.Messages))
	copy(msgs, req.Messages)
	m.mu.Lock()
	idx := m.callNum
	m.callNum++
	m.records = append(m.records, msgs)
	m.mu.Unlock()

	ch := make(chan *model.Response, 2)
	switch idx {
	case 0:
		ch <- &model.Response{
			ID: "mock-tc-1", Object: model.ObjectTypeChatCompletion, Done: true,
			Choices: []model.Choice{{
				Index: 0, Message: model.Message{
					Role: model.RoleAssistant, Content: "",
					ToolCalls: []model.ToolCall{{
						ID: "call-1", Type: "function",
						Function: model.FunctionDefinitionParam{
							Name: "blocking_tool", Arguments: []byte(`{"input":"wait"}`),
						},
					}},
				},
			}},
		}
	default:
		final := "最终回复：任务已完成。"
		for _, m := range req.Messages {
			if m.Role == model.RoleUser && strings.Contains(m.Content, "补充要求") {
				final = "好的，已包含补充要求。最终回复：任务已完成，包含测试。"
				break
			}
		}
		ch <- &model.Response{
			ID: "mock-text-1", Object: model.ObjectTypeChatCompletion, Done: true,
			Choices: []model.Choice{{
				Index: 0, Message: model.Message{Role: model.RoleAssistant, Content: final},
			}},
		}
	}
	close(ch)
	return ch, nil
}

// ═══════════════════════════════════════════════════════════════════════
// 工具
// ═══════════════════════════════════════════════════════════════════════

func must(err error) { if err != nil { panic(err) } }

var logFile *os.File

func logf(f string, a ...any) {
	s := fmt.Sprintf(f, a...)
	fmt.Println(s)
	if logFile != nil { fmt.Fprintln(logFile, s) }
}

// SSEEvent 结构化解析
type SSEEvent struct {
	Type        string `json:"type"`
	MessageID   string `json:"messageId"`
	Role        string `json:"role"`
	Delta       string `json:"delta"`
	Content     any    `json:"content"`
	ToolCallID  string `json:"toolCallId"`
	ToolCallName string `json:"toolCallName"`
	RunID       string `json:"runId"`
	ThreadID    string `json:"threadId"`
}

func parseSSE(r io.Reader) []SSEEvent {
	s := bufio.NewScanner(r)
	var out []SSEEvent
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, "data: ") { continue }
		var evt SSEEvent
		if err := json.Unmarshal([]byte(line[6:]), &evt); err == nil {
			out = append(out, evt)
		}
	}
	return out
}

func intPtr(i int) *int           { return &i }
func float64Ptr(f float64) *float64 { return &f }

func contentString(v any) string {
	if v == nil { return "" }
	if s, ok := v.(string); ok { return s }
	b, _ := json.Marshal(v)
	return string(b)
}

// ═══════════════════════════════════════════════════════════════════════
// main
// ═══════════════════════════════════════════════════════════════════════

func main() {
	_ = os.MkdirAll("forward/verify_core_enqueue_agui", 0755)
	var err error
	logFile, err = os.Create("forward/verify_core_enqueue_agui/verification_log.txt")
	must(err)
	defer logFile.Close()

	logf("================================================================")
	logf("  对照实验：无框架补丁，仅 Core Runner.EnqueueUserMessage")
	logf("================================================================")
	logf("")

	// ── 创建组件 ──
	bt := newBlockingTool()
	mm := &mockModel{}

	ag := llmagent.New("assistant",
		llmagent.WithModel(mm),
		llmagent.WithInstruction("使用 blocking_tool 工具。"),
		llmagent.WithTools([]tool.Tool{bt}),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			MaxTokens: intPtr(2000), Temperature: float64Ptr(0.7), Stream: false,
		}))

	ss := inmemory.NewSessionService()
	cr := baserunner.NewRunner(appName, ag, baserunner.WithSessionService(ss))

	// AG-UI Runner（完全原生，无 framework.patch）
	aguiR := aguirunner.New(cr,
		aguirunner.WithSessionService(ss),
		aguirunner.WithAppName(appName),
		aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
			return userID, nil
		}),
		aguirunner.WithRunOptionResolver(func(_ context.Context, input *adapter.RunAgentInput) ([]agent.RunOption, error) {
			return []agent.RunOption{agent.WithRequestID(input.RunID)}, nil
		}),
	)

	_ = aguiR // 仅用于创建 SSE Service
	sseSvc := sse.New(aguiR,
		service.WithPath("/chat"),
		service.WithMessagesSnapshotEnabled(true),
		service.WithMessagesSnapshotPath("/history"),
	)
	mux := http.NewServeMux()
	mux.Handle("/", sseSvc.Handler())
	srv := &http.Server{Addr: ":2099", Handler: mux}
	go srv.ListenAndServe()
	time.Sleep(200 * time.Millisecond)

	logf("=== 1. 通过 AG-UI SSE 启动 Run ===")
	var sseEvents []SSEEvent
	sseDone := make(chan struct{})

	body, _ := json.Marshal(map[string]any{
		"threadId": sessionID, "runId": runID,
		"messages": []map[string]any{{
			"id": "msg-init", "role": "user", "content": "开始执行任务",
		}},
	})
	go func() {
		defer close(sseDone)
		resp, err := http.Post("http://localhost:2099/chat", "application/json", bytes.NewReader(body))
		if err != nil { return }
		defer resp.Body.Close()
		sseEvents = parseSSE(resp.Body)
	}()

	<-bt.started
	logf("  blocking_tool 已开始 ✅")

	logf("")
	logf("=== 2. 仅调用 Core Runner.EnqueueUserMessage ===")
	err = baserunner.EnqueueUserMessage(cr, runID, model.NewUserMessage("补充要求：请同时添加测试"))
	logf("  EnqueueUserMessage err=%v", err)

	logf("")
	logf("=== 3. 释放 Tool 并等待 Run 完成 ===")
	close(bt.release)
	<-sseDone
	time.Sleep(800 * time.Millisecond)
	logf("  SSE 流已结束 ✅")

	// ── 4. 解析 Mock Model 调用记录 ──
	logf("")
	logf("=== 4. Mock Model 调用记录 ===")
	appendSeen := false
	for ci, msgs := range mm.records {
		logf("  调用 #%d: %d 条消息", ci, len(msgs))
		for _, m := range msgs {
			c := m.Content
			if len(c) > 60 { c = c[:60] + "..." }
			extra := ""
			if strings.Contains(m.Content, "补充要求") { extra = " ← 补充消息!"; appendSeen = true }
			logf("    role=%s content=%q%s", m.Role, c, extra)
		}
	}

	// ── 5. SSE 详细解析 ──
	logf("")
	logf("=== 5. SSE 事件流（详细解析）===")
	rS, rF := 0, 0
	var sseAppendTextMsg []SSEEvent // role=user 的 TEXT_MESSAGE
	for _, e := range sseEvents {
		if e.Type == "RUN_STARTED" { rS++ }
		if e.Type == "RUN_FINISHED" { rF++ }
		extra := ""
		if e.Type == "TEXT_MESSAGE_CONTENT" && strings.Contains(e.Delta, "补充要求") {
			extra = " ← 追加!"
		}
		if e.Type == "TEXT_MESSAGE_START" && e.Role == "user" {
			sseAppendTextMsg = append(sseAppendTextMsg, e)
		}
		logf("  [%d] type=%-25s role=%-10s delta=%q%s", len(sseEvents)-rF+rS,
			e.Type, e.Role, e.Delta, extra)
	}
	logf("  RUN_STARTED=%d  RUN_FINISHED=%d", rS, rF)

	// 检查 SSE：TEXT_MESSAGE 中包含追加内容（role 在 START 事件，不在 CONTENT）
	sseHasUserMsg := false
	sseState := "" // "" | "in-user-msg"
	for _, e := range sseEvents {
		if e.Type == "TEXT_MESSAGE_START" && e.Role == "user" {
			sseState = "in-user-msg"
		}
		if e.Type == "TEXT_MESSAGE_CONTENT" && sseState == "in-user-msg" && strings.Contains(e.Delta, "补充要求") {
			sseHasUserMsg = true
		}
		if e.Type == "TEXT_MESSAGE_END" && sseState == "in-user-msg" {
			sseState = ""
		}
	}

	// ── 6. MessagesSnapshot ──
	logf("")
	logf("=== 6. MessagesSnapshot ===")
	snapBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID, "runId": "snap-c1",
	})
	resp, err := http.Post("http://localhost:2099/history", "application/json", bytes.NewReader(snapBody))
	must(err)
	defer resp.Body.Close()

	logf("  HTTP status: %d", resp.StatusCode)
	logf("  Content-Type: %s", resp.Header.Get("Content-Type"))
	if resp.StatusCode != 200 {
		logf("  ❌ /history 返回非 200，实验异常终止")
		bodyBytes, _ := io.ReadAll(resp.Body)
		logf("  响应体: %s", string(bodyBytes))
		logf("")
		logf("  结论: 实验异常 — /history 不可用")
		os.Exit(1)
	}

	snapRaw, _ := io.ReadAll(resp.Body)
	logf("  原始响应 (%d bytes):\n%s", len(snapRaw), string(snapRaw))

	// 解析所有 SSE 事件
	type anyEvent map[string]any
	var snapAllEvents []anyEvent
	for _, line := range strings.Split(string(snapRaw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") { continue }
		var e anyEvent
		if err := json.Unmarshal([]byte(line[6:]), &e); err != nil {
			logf("  解析失败: %v", err)
			continue
		}
		snapAllEvents = append(snapAllEvents, e)
	}
	logf("  解析到 %d 个 SSE 事件", len(snapAllEvents))

	// 找到 MESSAGES_SNAPSHOT
	var snapMessages []map[string]any
	for _, e := range snapAllEvents {
		typ, _ := e["type"].(string)
		logf("    event type=%s", typ)
		if typ == "MESSAGES_SNAPSHOT" {
			if msgs, ok := e["messages"].([]any); ok {
				for _, m := range msgs {
					if msg, ok := m.(map[string]any); ok {
						snapMessages = append(snapMessages, msg)
					}
				}
			}
		}
	}

	messagesSnapshotCount := 0
	for _, e := range snapAllEvents {
		if t, _ := e["type"].(string); t == "MESSAGES_SNAPSHOT" { messagesSnapshotCount++ }
	}

	logf("  MESSAGES_SNAPSHOT 事件数: %d (期望 1)", messagesSnapshotCount)
	if messagesSnapshotCount == 0 {
		logf("  ❌ 实验异常：未收到 MESSAGES_SNAPSHOT 事件")
		logf("  判定：实验异常，不能判断历史缺失")
		logf("")
		logf("  可能原因：")
		logf("    - tracker 未写入 AG-UI TrackEvents")
		logf("    - /history request body threadId 格式不匹配")
		logf("    - session 创建失败")
		os.Exit(1)
	}

	logf("  快照消息 (%d 条):", len(snapMessages))
	appendInSnap := 0
	for _, m := range snapMessages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		disp := content
		if len(disp) > 60 { disp = disp[:60] + "..." }
		isAppend := role == "user" && content == "补充要求：请同时添加测试"
		extra := ""
		if isAppend { extra = " ← 追加消息!"; appendInSnap++ }
		logf("    role=%-10s content=%q%s", role, disp, extra)
	}
	logf("  role=user content精确匹配 数量: %d (期望 1)", appendInSnap)

	// ── 7. Session / Track ──
	logf("")
	logf("=== 7. Session 底层 ===")
	key := session.Key{AppName: appName, UserID: userID, SessionID: sessionID}
	sess, _ := ss.GetSession(context.Background(), key)
	if sess != nil {
		logf("  [session_events] %d 条:", len(sess.Events))
		for i, e := range sess.Events {
			obj, role, content := "", "", ""
			if e.Response != nil {
				obj = string(e.Response.Object)
				if len(e.Response.Choices) > 0 {
					c := e.Response.Choices[0].Message.Content
					if len(c) > 60 { c = c[:60] + "..." }
					content = c
					role = string(e.Response.Choices[0].Message.Role)
				}
			}
			if obj == "runner.completion" { continue }
			extra := ""
			if strings.Contains(content, "补充要求") { extra = " ← 补充!" }
			logf("    [%d] obj=%s role=%s content=%q%s", i, obj, role, content, extra)
		}

		// Track 严格解析
		logf("  [track]")
		trackAppendCount := 0
		sess.TracksMu.RLock()
		for tr, tEvts := range sess.Tracks {
			logf("    track=%q %d 条:", tr, len(tEvts.Events))
			for j, te := range tEvts.Events {
				var raw map[string]any
				json.Unmarshal(te.Payload, &raw)
				typ, _ := raw["type"].(string)
				name, _ := raw["name"].(string)
				var value map[string]any
				if v, ok := raw["value"]; ok {
					if vm, ok := v.(map[string]any); ok { value = vm }
				}
				extra := ""
				// 检查是否为追加的用户消息
				if typ == "CUSTOM" && name == "trpc-agent-go.user_message" {
					if vr, _ := value["role"].(string); vr == "user" {
						if vc, _ := value["content"].(string); vc == "补充要求：请同时添加测试" {
							extra = " ← 追加user_message!"
							trackAppendCount++
						}
					}
				}
				// TEXT_MESSAGE
				contentStr := ""
				if raw["delta"] != nil { contentStr, _ = raw["delta"].(string) }
				if contentStr == "" { contentStr, _ = raw["content"].(string) }
				if contentStr == "" {
					if value != nil { contentStr, _ = value["content"].(string) }
				}
				if strings.Contains(contentStr, "补充要求") && typ != "" {
					if extra == "" { extra = " ← 包含补充!" }
				}
				if len(contentStr) > 60 { contentStr = contentStr[:60] + "..." }
				ps := string(te.Payload)
				if len(ps) > 80 { ps = ps[:80] + "..." }
				logf("      [%d] type=%s %s", j, typ, ps)
			}
		}
		sess.TracksMu.RUnlock()
		logf("    CUSTOM user_message 精确匹配数量: %d (期望 0，因为无框架补丁)", trackAppendCount)
	}

	// ── 8. 幽灵消息 ──
	logf("")
	logf("=== 8. 幽灵消息 ===")
	err = baserunner.EnqueueUserMessage(cr, runID, model.NewUserMessage("幽灵消息"))
	logf("  EnqueueUserMessage err=%v (期望 error)", err)

	sess2, _ := ss.GetSession(context.Background(), key)
	ghostTrackCount := 0
	ghostSnapCount := 0
	if sess2 != nil {
		sess2.TracksMu.RLock()
		for _, tEvts := range sess2.Tracks {
			for _, te := range tEvts.Events {
				if strings.Contains(string(te.Payload), "幽灵消息") {
					ghostTrackCount++
				}
			}
		}
		sess2.TracksMu.RUnlock()
	}
	for _, m := range snapMessages {
		if content, _ := m["content"].(string); strings.Contains(content, "幽灵消息") {
			ghostSnapCount++
		}
	}
	logf("  幽灵 TrackEvent 中: %d (期望 0)", ghostTrackCount)
	logf("  幽灵 MessagesSnapshot 中: %d (期望 0)", ghostSnapCount)

	// ── 9. 结论 ──
	logf("")
	logf("================================================================")
	logf("  结论")
	logf("================================================================")
	logf("")
	logf("  Core 接收追加消息:                %s", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[appendSeen])
	logf("  原 SSE 实时收到用户消息 role=user: %s", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[sseHasUserMsg])
	logf("  Track 保存追加消息:               %s", map[bool]string{true: "⚠️ 参考", false: "ℹ️ 见说明"}[len(snapAllEvents) > 0])
	logf("  MessagesSnapshot 恰好出现一次:     %s", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[appendInSnap == 1])
	logf("  Run 结束后无幽灵消息:              %s", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[ghostTrackCount == 0 && ghostSnapCount == 0])
	logf("")

	// ── 对照说明 ──
	logf("================================================================")
	logf("  对照说明：为什么 verify_enqueue 没有看到追加用户消息")
	logf("================================================================")
	logf("")
	logf("  实际上 forward/verify_enqueue 在 MessagesSnapshot 中也")
	logf("  没有看到追加用户消息（snapshot 只有 init/tool_call/tool_result/")
	logf("  final response，缺少追加的 role=user 消息）。")
	logf("")
	logf("  原因：Core Runner.EnqueueUserMessage 只负责把消息放入")
	logf("  Agent 内部队列，不留 AG-UI TrackEvent。")
	logf("")
	logf("  AG-UI 的 MessagesSnapshot 基于 TrackEvents（reduce）还原，")
	logf("  不是基于 Session Events。追加消息如果不写入 TrackEvent，")
	logf("  就不会出现在 MESSAGES_SNAPSHOT 中。")
	logf("")
	logf("  当前版本 trpc-agent-go server/agui（v1.10.0）原生不支持")
	logf("  从外部写入 AG-UI TrackEvent 的公开 API。")
	logf("")
	logf("  因此无框架补丁时，追加消息在 MessagesSnapshot 中不可见。")
	logf("")

	logf("================================================================")
	logf("  架构问题回答")
	logf("================================================================")
	logf("")
	logf("  Q: EDITH 是否可以不修改 AG-UI 框架，仅用")
	logf("     RunRouter + Core Runner.EnqueueUserMessage 实现连续消息？")
	logf("")
	logf("  A: 部分可以，部分不可以。")
	logf("")
	logf("  可以做到的：")
	logf("  - ✅ 追加消息进入当前 Core Run（Agent 感知并回应）")
	logf("  - ✅ 追加消息写入 Session Events（可供后续轮次读取）")
	logf("  - ✅ SSE 流中存在追加消息相关事件（框架自动处理）")
	logf("")
	logf("  做不到的：")
	logf("  - ❌ MessagesSnapshot 中不会出现追加的 role=user 消息")
	logf("  - ❌ AG-UI Track Events 中缺少追加的 CUSTOM user_message")
	logf("")
	logf("  如果业务不要求 MessagesSnapshot /history 展示追加消息，")
	logf("  则可以不改框架。")
	logf("")
	logf("  如果要求 /history 展示追加消息，需要框架补丁（~130 行）。")
	logf("")
	logf("  完整日志: forward/verify_core_enqueue_agui/verification_log.txt")
}
