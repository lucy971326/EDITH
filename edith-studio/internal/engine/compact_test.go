package engine

import (
	"context"
	"testing"

	"edith/studio/internal/models"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"
)

type recordingSummaryService struct {
	frameworksession.Service
	storedSession *frameworksession.Session
	force         bool
	filterKey     string
	selection     models.Selection
}

func (s *recordingSummaryService) GetSession(
	context.Context,
	frameworksession.Key,
	...frameworksession.Option,
) (*frameworksession.Session, error) {
	return s.storedSession, nil
}

func (s *recordingSummaryService) CreateSessionSummary(
	ctx context.Context,
	_ *frameworksession.Session,
	filterKey string,
	force bool,
) error {
	s.force = force
	s.filterKey = filterKey
	s.selection, _ = models.SelectionFromContext(ctx)
	return nil
}

func TestCompactUsesTheCurrentModelSelection(t *testing.T) {
	summaryService := &recordingSummaryService{
		Service:       noop.NewService(),
		storedSession: frameworksession.NewSession("edith-studio", "workspace:test", "session-1"),
	}
	engine := newTestEngineWithSessionService(t, &recordingRunner{sourceEventCh: make(chan *event.Event)}, summaryService)

	err := engine.Compact(context.Background(), CompactInput{
		SessionID:    "session-1",
		ModelID:      "deepseek-test",
		ThinkingMode: "high",
	})
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if !summaryService.force {
		t.Fatal("summary was not forced")
	}
	if summaryService.filterKey != "edith-studio" {
		t.Fatalf("summary filter key = %q, want edith-studio", summaryService.filterKey)
	}
	if summaryService.selection.ModelID != "deepseek-test" || summaryService.selection.ThinkingMode != "high" {
		t.Fatalf("summary selection = %#v", summaryService.selection)
	}
}

func TestCompactRejectsAnActiveRun(t *testing.T) {
	sourceEventCh := make(chan *event.Event)
	runStartedCh := make(chan struct{})
	runner := &recordingRunner{sourceEventCh: sourceEventCh, runStartedCh: runStartedCh}
	engine := newTestEngine(t, runner)

	firstRunDoneCh := make(chan error, 1)
	go func() {
		firstRunDoneCh <- engine.Run(
			context.Background(),
			RunInput{RequestID: "request-1", SessionID: "session-1", Message: "first"},
			func(StreamEvent) error { return nil },
		)
	}()
	<-runStartedCh

	err := engine.Compact(context.Background(), CompactInput{SessionID: "session-1"})
	if err != ErrSessionBusy {
		t.Fatalf("compact error = %v, want ErrSessionBusy", err)
	}

	sourceEventCh <- responseEvent(model.Response{Object: model.ObjectTypeRunnerCompletion, Done: true})
	close(sourceEventCh)
	if err := <-firstRunDoneCh; err != nil {
		t.Fatalf("first run: %v", err)
	}
}
