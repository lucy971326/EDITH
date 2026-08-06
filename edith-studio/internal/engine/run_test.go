package engine

import (
	"context"
	"errors"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

type recordingRunner struct {
	sourceEventCh   chan *event.Event
	runStartedCh    chan struct{}
	runOptions      agent.RunOptions
	canceledRequest string
}

func (r *recordingRunner) Run(
	_ context.Context,
	_ string,
	_ string,
	_ model.Message,
	runOptions ...agent.RunOption,
) (<-chan *event.Event, error) {
	for _, runOption := range runOptions {
		runOption(&r.runOptions)
	}
	if r.runStartedCh != nil {
		close(r.runStartedCh)
	}
	return r.sourceEventCh, nil
}

func (r *recordingRunner) Close() error { return nil }

func (r *recordingRunner) Cancel(requestID string) bool {
	r.canceledRequest = requestID
	return true
}

func (r *recordingRunner) RunStatus(string) (runner.RunStatus, bool) {
	return runner.RunStatus{}, false
}

func TestRunStreamsFrameworkEventsWithoutAnIntermediateChannel(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 5)
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{
		Delta: model.Message{ReasoningContent: "plan", Content: "answer"},
	}}})
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{
		Message: model.Message{ToolCalls: []model.ToolCall{{
			ID:       "call-1",
			Function: model.FunctionDefinitionParam{Name: "read_file", Arguments: []byte(`{"path":"main.go"}`)},
		}}},
	}}})
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{
		Message: model.Message{ToolID: "call-1", Content: "file contents"},
	}}})
	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)

	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{
		ProjectRoot:    t.TempDir(),
		Runner:         runner,
		SessionService: inmemory.NewSessionService(),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var events []StreamEvent
	err = engine.Run(context.Background(), RunInput{RequestID: "request-1", SessionID: "session-1", Message: "hello"}, func(streamEvent StreamEvent) error {
		events = append(events, streamEvent)
		return nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	wantTypes := []string{"run.started", "reasoning.delta", "message.delta", "tool.started", "tool.finished", "run.completed"}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(wantTypes), events)
	}
	for index, wantType := range wantTypes {
		if events[index].Type != wantType {
			t.Fatalf("event %d type = %q, want %q", index, events[index].Type, wantType)
		}
	}
	if runner.runOptions.MaxRunDuration != maxRunDuration || runner.runOptions.DetachedCancel {
		t.Fatalf("run options = %#v, want application-bound 60-minute run", runner.runOptions)
	}
	if runner.runOptions.Stream == nil || !*runner.runOptions.Stream {
		t.Fatalf("run stream option = %#v, want true", runner.runOptions.Stream)
	}
	if events[3].ToolName != "read_file" || events[4].ToolResult != "file contents" {
		t.Fatalf("tool events = %#v %#v", events[3], events[4])
	}
}

func TestCancelDelegatesToManagedRunner(t *testing.T) {
	sourceEventCh := make(chan *event.Event)
	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{
		ProjectRoot:    t.TempDir(),
		Runner:         runner,
		SessionService: inmemory.NewSessionService(),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if !engine.Cancel("request-1") || runner.canceledRequest != "request-1" {
		t.Fatalf("cancel did not delegate: %q", runner.canceledRequest)
	}
}

func TestRunReportsUserCancellationAtCompletion(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 1)
	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{
		ProjectRoot:    t.TempDir(),
		Runner:         runner,
		SessionService: inmemory.NewSessionService(),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if !engine.Cancel("request-1") {
		t.Fatal("cancel returned false")
	}
	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)

	var streamEvents []StreamEvent
	err = engine.Run(context.Background(), RunInput{RequestID: "request-1", SessionID: "session-1", Message: "hello"}, func(streamEvent StreamEvent) error {
		streamEvents = append(streamEvents, streamEvent)
		return nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := streamEvents[len(streamEvents)-1].Type; got != "run.canceled" {
		t.Fatalf("terminal event = %q, want run.canceled", got)
	}
}

func TestRunHandlesCompleteResponseAndDeduplicatesTools(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 4)
	toolCall := model.ToolCall{ID: "call-1", Function: model.FunctionDefinitionParam{Name: "Read"}}
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{Message: model.Message{ReasoningContent: "plan", Content: "answer"}}}})
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{Message: model.Message{ToolCalls: []model.ToolCall{toolCall}}}}})
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{Message: model.Message{ToolCalls: []model.ToolCall{toolCall}}}}})
	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)

	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{ProjectRoot: t.TempDir(), Runner: runner, SessionService: inmemory.NewSessionService()})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var streamEvents []StreamEvent
	err = engine.Run(context.Background(), RunInput{RequestID: "request-1", SessionID: "session-1", Message: "hello"}, func(streamEvent StreamEvent) error {
		streamEvents = append(streamEvents, streamEvent)
		return nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var reasoningCount, textCount, toolStartedCount int
	for _, streamEvent := range streamEvents {
		switch streamEvent.Type {
		case "reasoning.delta":
			reasoningCount++
		case "message.delta":
			textCount++
		case "tool.started":
			toolStartedCount++
		}
	}
	if reasoningCount != 1 || textCount != 1 || toolStartedCount != 1 {
		t.Fatalf("counts reasoning=%d text=%d tool.started=%d", reasoningCount, textCount, toolStartedCount)
	}
}

func TestRunRejectsASecondRunInTheSameSession(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 1)
	runStartedCh := make(chan struct{})
	runner := &recordingRunner{sourceEventCh: sourceEventCh, runStartedCh: runStartedCh}
	engine, err := New(Dependencies{ProjectRoot: t.TempDir(), Runner: runner, SessionService: inmemory.NewSessionService()})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	firstRunDoneCh := make(chan error, 1)
	go func() {
		firstRunDoneCh <- engine.Run(
			context.Background(),
			RunInput{RequestID: "request-1", SessionID: "session-1", Message: "first"},
			func(StreamEvent) error { return nil },
		)
	}()
	<-runStartedCh

	err = engine.Run(
		context.Background(),
		RunInput{RequestID: "request-2", SessionID: "session-1", Message: "second"},
		func(StreamEvent) error { return nil },
	)
	if !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("second run error = %v, want ErrSessionBusy", err)
	}

	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)
	if err := <-firstRunDoneCh; err != nil {
		t.Fatalf("first run: %v", err)
	}
}

func TestRunKeepsConsumingAfterTheBrowserDisconnects(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 2)
	sourceEventCh <- responseEvent(model.Response{Choices: []model.Choice{{Delta: model.Message{Content: "answer"}}}})
	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)

	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{ProjectRoot: t.TempDir(), Runner: runner, SessionService: inmemory.NewSessionService()})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sendCalls := 0
	err = engine.Run(
		context.Background(),
		RunInput{RequestID: "request-1", SessionID: "session-1", Message: "hello"},
		func(StreamEvent) error {
			sendCalls++
			return errors.New("browser disconnected")
		},
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if sendCalls != 1 {
		t.Fatalf("send calls = %d, want 1", sendCalls)
	}
	if len(sourceEventCh) != 0 {
		t.Fatalf("framework event channel still contains %d events", len(sourceEventCh))
	}
}

func TestRunReturnsTheFrameworkErrorMessage(t *testing.T) {
	sourceEventCh := make(chan *event.Event, 2)
	sourceEventCh <- responseEvent(model.Response{
		Object: model.ObjectTypeError,
		Error:  &model.ResponseError{Message: "model unavailable"},
	})
	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)

	runner := &recordingRunner{sourceEventCh: sourceEventCh}
	engine, err := New(Dependencies{ProjectRoot: t.TempDir(), Runner: runner, SessionService: inmemory.NewSessionService()})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var streamEvents []StreamEvent
	err = engine.Run(
		context.Background(),
		RunInput{RequestID: "request-1", SessionID: "session-1", Message: "hello"},
		func(streamEvent StreamEvent) error {
			streamEvents = append(streamEvents, streamEvent)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(streamEvents) != 2 || streamEvents[1].Type != "run.error" {
		t.Fatalf("stream events = %#v, want run.started then run.error", streamEvents)
	}
	if streamEvents[1].Error == nil || streamEvents[1].Error.Message != "model unavailable" {
		t.Fatalf("stream error = %#v, want framework message", streamEvents[1].Error)
	}
}

func responseEvent(response model.Response) *event.Event {
	return event.NewResponseEvent("invocation", "edith", &response)
}

var _ runner.ManagedRunner = (*recordingRunner)(nil)
