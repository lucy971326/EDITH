// Package usage owns EDITH's durable Agent token accounting.
package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	statusRunning   = "running"
	statusCompleted = "completed"
	statusFailed    = "failed"
)

// Run identifies one framework Invocation. RequestID is the framework's
// request identity; EDITH does not introduce a second run ID.
type Run struct {
	RequestID string
	UserID    string
	SessionID string
	ModelID   string
}

// Tokens is the provider-reported usage accumulated across one Agent Run.
// CachedPromptTokens is nil when this model/provider does not report it.
type Tokens struct {
	PromptTokens       int
	CachedPromptTokens *int
	CompletionTokens   int
	TotalTokens        int
}

// AddTokens includes one complete model response. Partial streaming chunks
// must not reach this function because their usage values can be cumulative.
func AddTokens(tokens *Tokens, source *model.Usage, reportsCachedPromptTokens bool) {
	if source == nil {
		return
	}
	tokens.PromptTokens += source.PromptTokens
	tokens.CompletionTokens += source.CompletionTokens
	tokens.TotalTokens += source.TotalTokens
	if !reportsCachedPromptTokens {
		return
	}
	if tokens.CachedPromptTokens == nil {
		tokens.CachedPromptTokens = new(int)
	}
	*tokens.CachedPromptTokens += int(source.PromptTokensDetails.CachedTokens)
}

// Summary is the browser-facing usage aggregate for one conversation.
// Nil cache fields mean at least one completed run did not report cache data.
type Summary struct {
	TotalTokens          int      `json:"totalTokens"`
	CachedPromptTokens   *int     `json:"cachedPromptTokens"`
	UncachedPromptTokens *int     `json:"uncachedPromptTokens"`
	CompletionTokens     int      `json:"completionTokens"`
	CacheHitRate         *float64 `json:"cacheHitRate"`
}

// CreateTable creates Usage tables on EDITH's caller-owned SQLite database.
func CreateTable(db *sql.DB) error {
	if db == nil {
		return errors.New("usage database is required")
	}
	return createTable(db, context.Background())
}

// Start records one accepted Agent Run before its event stream is consumed.
func Start(db *sql.DB, ctx context.Context, run Run) error {
	run = normalizeRun(run)
	if run.RequestID == "" || run.UserID == "" || run.SessionID == "" || run.ModelID == "" {
		return errors.New("requestID, userID, sessionID, and modelID are required")
	}
	_, err := db.ExecContext(ctx, `INSERT INTO agent_runs (
		request_id, user_id, session_id, model_id, status
	) VALUES (?, ?, ?, ?, ?)`,
		run.RequestID, run.UserID, run.SessionID, run.ModelID, statusRunning,
	)
	if err != nil {
		return fmt.Errorf("start agent run %q: %w", run.RequestID, err)
	}
	return nil
}

// Finish stores one completed Run and returns the conversation Usage summary.
func Finish(db *sql.DB, ctx context.Context, run Run, tokens Tokens) (Summary, error) {
	requestID := strings.TrimSpace(run.RequestID)
	cacheReported := tokens.CachedPromptTokens != nil
	var cached any
	if cacheReported {
		cached = *tokens.CachedPromptTokens
	}
	result, err := db.ExecContext(ctx, `UPDATE agent_runs
		SET prompt_tokens = ?, cached_prompt_tokens = ?, cache_reported = ?,
			completion_tokens = ?, total_tokens = ?, status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE request_id = ? AND status = ?`,
		tokens.PromptTokens, cached, cacheReported, tokens.CompletionTokens,
		tokens.TotalTokens, statusCompleted, requestID, statusRunning,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("finish agent run %q: %w", requestID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Summary{}, fmt.Errorf("finish agent run %q: run not found", requestID)
	}
	return SessionSummary(db, ctx, run.UserID, run.SessionID)
}

// Fail marks an unfinished Agent Run as failed without contributing it to
// conversation totals.
func Fail(db *sql.DB, ctx context.Context, requestID string) error {
	_, err := db.ExecContext(ctx, `UPDATE agent_runs
		SET status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE request_id = ? AND status = ?`,
		statusFailed, strings.TrimSpace(requestID), statusRunning,
	)
	return err
}

// SessionSummary returns the durable, server-calculated token aggregate for
// one conversation. Failed runs are deliberately excluded.
func SessionSummary(db *sql.DB, ctx context.Context, userID, sessionID string) (Summary, error) {
	var row struct {
		Count      int
		Total      int
		Completion int
		Prompt     int
		CacheKnown int
		Cached     sql.NullInt64
	}
	err := db.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(cache_reported), 0),
		SUM(cached_prompt_tokens)
		FROM agent_runs
		WHERE user_id = ? AND session_id = ? AND status = ?`,
		strings.TrimSpace(userID), strings.TrimSpace(sessionID), statusCompleted,
	).Scan(&row.Count, &row.Total, &row.Completion, &row.Prompt, &row.CacheKnown, &row.Cached)
	if err != nil {
		return Summary{}, fmt.Errorf("summarize session usage: %w", err)
	}

	summary := Summary{TotalTokens: row.Total, CompletionTokens: row.Completion}
	if row.Count == 0 || row.CacheKnown != row.Count || !row.Cached.Valid {
		return summary, nil
	}
	cached := int(row.Cached.Int64)
	uncached := row.Prompt - cached
	var rate float64
	if row.Prompt > 0 {
		rate = float64(cached) / float64(row.Prompt)
	}
	summary.CachedPromptTokens = &cached
	summary.UncachedPromptTokens = &uncached
	summary.CacheHitRate = &rate
	return summary, nil
}

func normalizeRun(run Run) Run {
	run.RequestID = strings.TrimSpace(run.RequestID)
	run.UserID = strings.TrimSpace(run.UserID)
	run.SessionID = strings.TrimSpace(run.SessionID)
	run.ModelID = strings.TrimSpace(run.ModelID)
	return run
}
