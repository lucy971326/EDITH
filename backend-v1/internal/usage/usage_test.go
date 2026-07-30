package usage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestSessionSummaryOnlyIncludesCompletedRunsWithKnownCacheUsage(t *testing.T) {
	db := openTestDatabase(t)

	first := Run{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := Start(db, context.Background(), first); err != nil {
		t.Fatal(err)
	}
	firstTokens := Tokens{}
	AddTokens(&firstTokens, &model.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120, PromptTokensDetails: model.PromptTokensDetails{CachedTokens: 80}}, true)
	if _, err := Finish(db, context.Background(), first, firstTokens); err != nil {
		t.Fatal(err)
	}

	second := Run{RequestID: "request-2", UserID: "user-1", SessionID: "session-1", ModelID: "model-2"}
	if err := Start(db, context.Background(), second); err != nil {
		t.Fatal(err)
	}
	secondTokens := Tokens{}
	AddTokens(&secondTokens, &model.Usage{PromptTokens: 50, CompletionTokens: 10, TotalTokens: 60}, false)
	if _, err := Finish(db, context.Background(), second, secondTokens); err != nil {
		t.Fatal(err)
	}

	failed := Run{RequestID: "request-3", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := Start(db, context.Background(), failed); err != nil {
		t.Fatal(err)
	}
	if err := Fail(db, context.Background(), failed.RequestID); err != nil {
		t.Fatal(err)
	}

	summary, err := SessionSummary(db, context.Background(), "user-1", "session-1")
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
	db := openTestDatabase(t)

	run := Run{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := Start(db, context.Background(), run); err != nil {
		t.Fatal(err)
	}
	tokens := Tokens{}
	AddTokens(&tokens, &model.Usage{PromptTokens: 100, CompletionTokens: 25, TotalTokens: 125, PromptTokensDetails: model.PromptTokensDetails{CachedTokens: 75}}, true)
	if _, err := Finish(db, context.Background(), run, tokens); err != nil {
		t.Fatal(err)
	}

	summary, err := SessionSummary(db, context.Background(), "user-1", "session-1")
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

func TestStartRejectsDuplicateRequestIDAndStatusIsUserScoped(t *testing.T) {
	db := openTestDatabase(t)
	run := Run{RequestID: "request-1", UserID: "user-1", SessionID: "session-1", ModelID: "model-1"}
	if err := Start(db, context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if err := Start(db, context.Background(), run); !errors.Is(err, ErrRunAlreadyExists) {
		t.Fatalf("duplicate start error = %v, want ErrRunAlreadyExists", err)
	}

	status, err := Status(db, context.Background(), "user-1", run.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if status != statusRunning {
		t.Fatalf("status = %q, want %q", status, statusRunning)
	}

	_, err = Status(db, context.Background(), "user-2", run.RequestID)
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("other user status error = %v, want ErrRunNotFound", err)
	}
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	err = CreateTable(db)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
