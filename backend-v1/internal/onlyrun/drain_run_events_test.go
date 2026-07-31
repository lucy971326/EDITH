package onlyrun

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"edith/backend-v1/internal/usage"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// newTestOnlyRun 构造一个只依赖内存 SQLite 的 OnlyRun。
// drainRunEvents 只用 usageDB、lanes 和 userCancelMarks，runner 等依赖留空即可。
func newTestOnlyRun(t *testing.T) *OnlyRun {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := usage.CreateTable(db); err != nil {
		t.Fatal(err)
	}
	return &OnlyRun{usageDB: db, lanes: newSessionLanes(), userCancelMarks: newUserCancelMarks()}
}

// runDrain 同步执行 drainRunEvents 并收集全部输出事件，便于逐项断言。
// 它先写入一条 running 状态的 agent_run 记录，模拟 OnlyRun.Run 的真实前置。
func runDrain(t *testing.T, o *OnlyRun, request MessageRequest, raw []*event.Event) []StreamEvent {
	t.Helper()
	run := usage.Run{RequestID: request.RequestID, UserID: request.UserID, SessionID: request.SessionID, ModelID: request.ModelID}
	if err := usage.Start(o.usageDB, context.Background(), run); err != nil {
		t.Fatalf("usage.Start: %v", err)
	}
	rawCh := make(chan *event.Event, len(raw))
	for _, item := range raw {
		rawCh <- item
	}
	close(rawCh)

	out := make(chan StreamEvent)
	var got []StreamEvent
	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range out {
			got = append(got, item)
		}
	}()
	o.drainRunEvents(context.Background(), request, false, run, rawCh, out, func() error { return nil })
	<-done
	return got
}

// wantEventTypes 按顺序断言输出事件类型。
func wantEventTypes(t *testing.T, got []StreamEvent, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("事件数量 = %d, 期望 %d; 实际: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Type != want[i] {
			t.Fatalf("事件[%d].Type = %q, 期望 %q; 全部: %#v", i, got[i].Type, want[i], got)
		}
	}
}

// textChunk 构造一个携带文本增量的事件。
func textChunk(content string) *event.Event {
	return &event.Event{Response: &model.Response{IsPartial: true, Choices: []model.Choice{{Delta: model.Message{Content: content}}}}}
}

// reasoningChunk 构造一个携带推理增量的事件。
func reasoningChunk(content string) *event.Event {
	return &event.Event{Response: &model.Response{IsPartial: true, Choices: []model.Choice{{Delta: model.Message{ReasoningContent: content}}}}}
}

// completionEvent 构造框架的收尾事件。
func completionEvent() *event.Event {
	return &event.Event{Response: &model.Response{Done: true, Object: model.ObjectTypeRunnerCompletion}}
}

func TestDrainRunEventsEmitsTextStream(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		textChunk("你"),
		textChunk("好"),
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "message.delta", "message.delta", "run.completed"})
	if got[1].Delta != "你" || got[2].Delta != "好" {
		t.Fatalf("delta 内容 = %q / %q, 期望 你 / 好", got[1].Delta, got[2].Delta)
	}
	if got[1].BlockType != "text" || got[1].BlockID == "" || got[1].BlockID != got[2].BlockID {
		t.Fatalf("连续文本块应共享同一 BlockID: %#v / %#v", got[1], got[2])
	}
}

func TestDrainRunEventsSplitsReasoningAndTextBlocks(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		reasoningChunk("思考1"),
		reasoningChunk("思考2"),
		textChunk("回答"),
		reasoningChunk("再想"),
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "reasoning.delta", "reasoning.delta", "message.delta", "reasoning.delta", "run.completed"})
	if got[1].BlockID != got[2].BlockID || got[1].BlockID != "assistant_request-1_reasoning_1" {
		t.Fatalf("连续推理块应共享 _reasoning_1: %#v / %#v", got[1], got[2])
	}
	if got[3].BlockID != "assistant_request-1_text_2" || got[3].BlockType != "text" {
		t.Fatalf("切换到文本块应生成 _text_2: %#v", got[3])
	}
	if got[4].BlockID != "assistant_request-1_reasoning_3" {
		t.Fatalf("切回推理块应生成 _reasoning_3: %#v", got[4])
	}
}

func TestDrainRunEventsEmitsToolLifecycle(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		{
			Response: &model.Response{
				IsPartial: true,
				Choices: []model.Choice{{
					Message: model.Message{ToolID: "tool-1", ToolName: "search", Content: `{"ok":true}`},
				}},
			},
		},
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "tool.started", "tool.finished", "run.completed"})
	if got[1].ToolCallID != "tool-1" || got[1].ToolName != "search" || got[1].ToolStatus != "running" {
		t.Fatalf("tool.started 字段异常: %#v", got[1])
	}
	if got[2].ToolStatus != "completed" || got[2].ToolResult != `{"ok":true}` {
		t.Fatalf("tool.finished 字段异常: %#v", got[2])
	}
}

func TestDrainRunEventsMarksFailedTool(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		{
			Response: &model.Response{
				IsPartial: true,
				Error:     &model.ResponseError{Message: "工具调用失败"},
				Choices: []model.Choice{{
					Message: model.Message{ToolID: "tool-1", ToolName: "search", Content: "boom"},
				}},
			},
		},
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "run.error", "tool.started", "tool.finished", "run.completed"})
	if got[3].ToolStatus != "failed" {
		t.Fatalf("tool.finished.ToolStatus = %q, 期望 failed", got[3].ToolStatus)
	}
}

func TestDrainRunEventsEmitsCanceledInsteadOfCompleted(t *testing.T) {
	o := newTestOnlyRun(t)
	o.userCancelMarks.mark("request-1")
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		textChunk("好"),
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "message.delta", "run.canceled"})
	if got[2].Usage == nil {
		t.Fatal("取消的任务也应有用量统计")
	}
	if o.userCancelMarks.marked("request-1") {
		t.Fatal("取消标记应在收尾后被消费")
	}
}

func TestDrainRunEventsCanceledSuppressesErrorEvent(t *testing.T) {
	o := newTestOnlyRun(t)
	o.userCancelMarks.mark("request-1")
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		{Response: &model.Response{Object: model.ObjectTypeError, Error: &model.ResponseError{Message: "boom"}, Done: true}},
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "run.canceled"})
}

func TestDrainRunEventsEmitsErrorEvent(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		{Response: &model.Response{Object: model.ObjectTypeError, Error: &model.ResponseError{Message: "boom"}, Done: true}},
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "run.error", "run.completed"})
	if got[1].Error == nil || got[1].Error.Type != "runner_error" || got[1].Error.Message != "boom" {
		t.Fatalf("run.error 字段异常: %#v", got[1])
	}
}

func TestDrainRunEventsAccumulatesUsage(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		{Response: &model.Response{Choices: []model.Choice{{Message: model.Message{Content: "a"}}}, Usage: &model.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}},
		{Response: &model.Response{Choices: []model.Choice{{Message: model.Message{Content: "b"}}}, Usage: &model.Usage{PromptTokens: 20, CompletionTokens: 8, TotalTokens: 28}}},
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "run.completed"})
	summary := got[1].Usage
	if summary == nil {
		t.Fatal("run.completed 缺少用量统计")
	}
	if summary.TotalTokens != 43 || summary.CompletionTokens != 13 {
		t.Fatalf("用量统计 = %#v, 期望 TotalTokens=43 CompletionTokens=13", summary)
	}
	if summary.CachedPromptTokens == nil || *summary.CachedPromptTokens != 0 {
		t.Fatalf("应报告缓存 token 且为 0: %#v", summary)
	}
}

func TestDrainRunEventsMarksFailedWhenStreamEndsAbruptly(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		textChunk("好"),
	})

	wantEventTypes(t, got, []string{"run.started", "message.delta"})
	status, err := usage.Status(o.usageDB, context.Background(), request.UserID, request.RequestID)
	if err != nil {
		t.Fatalf("查询状态: %v", err)
	}
	if status != "failed" {
		t.Fatalf("状态 = %q, 期望 failed", status)
	}
}

func TestDrainRunEventsSkipsNilEvents(t *testing.T) {
	o := newTestOnlyRun(t)
	request := MessageRequest{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "deepseek-v3"}
	got := runDrain(t, o, request, []*event.Event{
		nil,
		{Response: nil},
		textChunk("好"),
		completionEvent(),
	})

	wantEventTypes(t, got, []string{"run.started", "message.delta", "run.completed"})
}
