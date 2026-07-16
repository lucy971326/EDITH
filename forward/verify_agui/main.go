// Experiment: 验证 Web 与 GitHub 共用同一个 AG-UI Runner 时，AG-UI 历史能否统一展示
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
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	agentevent "trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/service"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/service/sse"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

const (
	appName   = "edith"
	userID    = "test-user"
	sessionID = "github:test/repo#1"
)

// ─── fakeRunner: 固定回复 Agent ─────────────────────────────────────
type fakeRunner struct {
	calls int
}

func (f *fakeRunner) Run(
	ctx context.Context,
	uid, sid string,
	msg model.Message,
	opts ...agent.RunOption,
) (<-chan *agentevent.Event, error) {
	f.calls++
	ch := make(chan *agentevent.Event, 2)
	ro := agent.NewRunOptions(opts...)

	inputContent := msg.Content
	if ro.Messages != nil {
		for i := len(ro.Messages) - 1; i >= 0; i-- {
			if ro.Messages[i].Role == model.RoleUser && ro.Messages[i].Content != "" {
				inputContent = ro.Messages[i].Content
				break
			}
		}
	}
	respContent := fmt.Sprintf("收到: %s", inputContent)

	ch <- &agentevent.Event{
		ID:           fmt.Sprintf("resp-%d", f.calls),
		InvocationID: ro.RequestID,
		Response: &model.Response{
			ID:     fmt.Sprintf("msg-%d", f.calls),
			Object: model.ObjectTypeChatCompletion,
			Done:   true,
			Choices: []model.Choice{{
				Index: 0,
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: respContent,
				},
				Delta: model.Message{Content: respContent},
			}},
		},
	}
	ch <- &agentevent.Event{
		Response: &model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true},
	}
	close(ch)
	return ch, nil
}

func (f *fakeRunner) Close() error { return nil }

// ─── 工具 ─────────────────────────────────────────────────────────────
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
	if err := scanner.Err(); err != nil {
		logf("  SSE scanner error: %v", err)
	}
	return evts
}

// ─── main ─────────────────────────────────────────────────────────────
func main() {
	var err error
	logFile, err = os.Create("forward/verification_log.txt")
	must(err)
	defer logFile.Close()

	logf("============================================")
	logf("  EDITH AG-UI 统一历史验证实验")
	logf("============================================")

	// ═════ 1. 搭建共享环境 ═════════════════════════════
	logf("\n--- 1. 搭建共享环境 ---")

	sessionService := inmemory.NewSessionService()
	coreRunner := &fakeRunner{}

	// 创建唯一的 sharedAGUIRunner
	sharedAGUIRunner := aguirunner.New(
		coreRunner,
		aguirunner.WithSessionService(sessionService),
		aguirunner.WithAppName(appName),
		aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
			return userID, nil
		}),
	)

	// 用 sharedAGUIRunner 直接创建 SSE 服务
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
	logf("  SSE 服务: http://localhost:2099/chat")
	logf("  appName=%s userID=%s sessionID=%s", appName, userID, sessionID)

	// ═════ 2. Web Run ── SSE HTTP ──────────────────────
	logf("\n--- 2. Web Run: SSE HTTP ---")

	webBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID,
		"runId":    "web-run-1",
		"messages": []map[string]any{{
			"id": "msg-web", "role": "user", "content": "这是 Web 消息",
		}},
	})

	resp, err := http.Post("http://localhost:2099/chat", "application/json",
		bytes.NewReader(webBody))
	must(err)
	webSSE := readSSE(resp.Body)
	resp.Body.Close()
	logf("  SSE 事件数: %d", len(webSSE))
	for i, evt := range webSSE {
		logf("    [%d] type=%s", i, evt["type"])
	}

	// ═════ 3. GitHub Run ── 直接调用 sharedAGUIRunner ──
	logf("\n--- 3. GitHub Run: sharedAGUIRunner.Run() ---")

	ghInput := &adapter.RunAgentInput{
		ThreadID: sessionID,
		RunID:    "github-run-1",
		Messages: []types.Message{{
			ID: "msg-gh", Role: types.RoleUser, Content: "这是 GitHub 消息",
		}},
	}
	eventsCh, err := sharedAGUIRunner.Run(context.Background(), ghInput)
	must(err)

	var ghAGUIEvents []aguievents.Event
	for evt := range eventsCh {
		ghAGUIEvents = append(ghAGUIEvents, evt)
	}
	logf("  AG-UI 事件数: %d", len(ghAGUIEvents))
	for i, evt := range ghAGUIEvents {
		base := evt.GetBaseEvent()
		logf("    [%d] type=%s", i, base.Type())
	}

	// 等待 tracker flush
	time.Sleep(800 * time.Millisecond)

	// ═════ 4. MessagesSnapshot 历史查询 ────────────────
	logf("\n--- 4. MessagesSnapshot ---")

	snapBody, _ := json.Marshal(map[string]any{
		"threadId": sessionID,
		"runId":    "snapshot-check",
	})
	resp2, err := http.Post("http://localhost:2099/history", "application/json",
		bytes.NewReader(snapBody))
	must(err)
	snapshotSSE := readSSE(resp2.Body)
	resp2.Body.Close()
	logf("  快照事件数: %d", len(snapshotSSE))
	for i, evt := range snapshotSSE {
		logf("    [%d] type=%s", i, evt["type"])
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			logJSON("    快照数据", evt)
		}
	}

	// ═════ 5. Session 底层数据检查 ─────────────────────
	logf("\n--- 5. Session 底层数据 ---")

	key := session.Key{AppName: appName, UserID: userID, SessionID: sessionID}
	sess, err := sessionService.GetSession(context.Background(), key)
	if err != nil {
		logf("  GetSession 错误: %v", err)
	} else if sess == nil {
		logf("  Session 不存在")
	} else {
		logf("  Session: app=%s user=%s id=%s", sess.AppName, sess.UserID, sess.ID)

		logf("\n  [session_events] 共 %d 条:", len(sess.Events))
		for i, e := range sess.Events {
			obj := ""
			role := ""
			content := ""
			if e.Response != nil {
				obj = string(e.Response.Object)
				if len(e.Response.Choices) > 0 {
					role = string(e.Response.Choices[0].Message.Role)
					content = e.Response.Choices[0].Message.Content
					if len(content) > 60 {
						content = content[:60] + "..."
					}
				}
			}
			if obj == "runner.completion" {
				continue
			}
			logf("    [%d] id=%s role=%s content=%q", i, e.ID, role, content)
		}

		logf("\n  [session_track_events]:")
		sess.TracksMu.RLock()
		for track, trackEvts := range sess.Tracks {
			logf("    track=%q 共 %d 条:", track, len(trackEvts.Events))
			for j, te := range trackEvts.Events {
				payloadStr := string(te.Payload)
				if len(payloadStr) > 120 {
					payloadStr = payloadStr[:120] + "..."
				}
				logf("      [%d] ts=%s payload=%s", j, te.Timestamp.Format("15:04:05.000"), payloadStr)
			}
		}
		sess.TracksMu.RUnlock()

		logf("\n  [Session State]:")
		if sess.State != nil {
			for k, v := range sess.State {
				if len(v) > 120 {
					logf("    %s=%s...", k, string(v[:120]))
				} else {
					logf("    %s=%s", k, string(v))
				}
			}
		} else {
			logf("    (nil)")
		}
	}

	// ═════ 6. 对照实验 ═══════════════════════════════
	logf("\n--- 6. 对照实验: 独立 Runner ---")

	sepSessService := inmemory.NewSessionService()
	sepAGUIRunner := aguirunner.New(
		&fakeRunner{},
		aguirunner.WithSessionService(sepSessService),
		aguirunner.WithAppName(appName),
		aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
			return userID, nil
		}),
	)
	sepEventsCh, err := sepAGUIRunner.Run(context.Background(), &adapter.RunAgentInput{
		ThreadID: sessionID,
		RunID:    "sep-run-1",
		Messages: []types.Message{{
			ID: "msg-sep", Role: types.RoleUser, Content: "这是另一个 Runner 的消息",
		}},
	})
	must(err)
	for range sepEventsCh {
	}

	origSess, _ := sessionService.GetSession(context.Background(), key)
	if origSess != nil {
		logf("  原始 Session 事件数: %d", len(origSess.Events))
	}
	sepSess, _ := sepSessService.GetSession(context.Background(), key)
	if sepSess != nil {
		logf("  分离 Session 事件数: %d (独立 SessionService)", len(sepSess.Events))
	}

	// ═════ 7. 结论 ─────────────────────────────────────
	logf("\n============================================")
	logf("  结论")
	logf("============================================")

	foundWeb := false
	foundGH := false
	for _, evt := range snapshotSSE {
		if evt["type"] == "MESSAGES_SNAPSHOT" {
			if messages, ok := evt["messages"].([]any); ok {
				for _, m := range messages {
					if msg, ok := m.(map[string]any); ok {
						if content, ok := msg["content"].(string); ok {
							if strings.Contains(content, "这是 Web 消息") || strings.Contains(content, "收到: 这是 Web 消息") {
								foundWeb = true
							}
							if strings.Contains(content, "这是 GitHub 消息") || strings.Contains(content, "收到: 这是 GitHub 消息") {
								foundGH = true
							}
						}
					}
				}
			}
		}
	}
	logf("  Web 消息在 MessagesSnapshot 中: %v", foundWeb)
	logf("  GitHub 消息在 MessagesSnapshot 中: %v", foundGH)
	logf("  完整日志: forward/verification_log.txt")
}
