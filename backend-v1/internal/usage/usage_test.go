package usage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestSessionSummaryOnlyIncludesCompletedRunsWithKnownCacheUsage(t *testing.T) {
	service := openTestService(t)

	first := Run{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := service.Start(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	firstTokens := Tokens{}
	firstTokens.Add(&model.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120, PromptTokensDetails: model.PromptTokensDetails{CachedTokens: 80}}, true)
	if err := service.Complete(context.Background(), first.RequestID, firstTokens); err != nil {
		t.Fatal(err)
	}

	second := Run{RequestID: "request-2", UserID: "user-1", SessionID: "session-1", ModelID: "model-2"}
	if err := service.Start(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	secondTokens := Tokens{}
	secondTokens.Add(&model.Usage{PromptTokens: 50, CompletionTokens: 10, TotalTokens: 60}, false)
	if err := service.Complete(context.Background(), second.RequestID, secondTokens); err != nil {
		t.Fatal(err)
	}

	failed := Run{RequestID: "request-3", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := service.Start(context.Background(), failed); err != nil {
		t.Fatal(err)
	}
	if err := service.Fail(context.Background(), failed.RequestID); err != nil {
		t.Fatal(err)
	}

	summary, err := service.SessionSummary(context.Background(), "user-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalTokens != 180 || summary.CompletionTokens != 30 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.CachedPromptTokens != nil || summary.UncachedPromptTokens != nil || summary.CacheHitRate != nil {
		t.Fatalf("unknown cache reporting must remain nil: %+v", summary)
	}
}

func TestSessionSummaryCalculatesCacheMetrics(t *testing.T) {
	service := openTestService(t)

	run := Run{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := service.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	tokens := Tokens{}
	tokens.Add(&model.Usage{PromptTokens: 100, CompletionTokens: 25, TotalTokens: 125, PromptTokensDetails: model.PromptTokensDetails{CachedTokens: 75}}, true)
	if err := service.Complete(context.Background(), run.RequestID, tokens); err != nil {
		t.Fatal(err)
	}

	summary, err := service.SessionSummary(context.Background(), "user-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if summary.CachedPromptTokens == nil || *summary.CachedPromptTokens != 75 {
		t.Fatalf("cached = %+v", summary.CachedPromptTokens)
	}
	if summary.UncachedPromptTokens == nil || *summary.UncachedPromptTokens != 25 {
		t.Fatalf("uncached = %+v", summary.UncachedPromptTokens)
	}
	if summary.CacheHitRate == nil || *summary.CacheHitRate != 0.75 {
		t.Fatalf("rate = %+v", summary.CacheHitRate)
	}
}

func openTestService(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	service, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
