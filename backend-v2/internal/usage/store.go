package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// store 是用量模块私有的 SQLite 访问器。
type store struct {
	db *sql.DB
}

func (s *store) createSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS usage_runs (
		request_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		model_id TEXT NOT NULL,
		prompt_tokens INTEGER,
		cached_prompt_tokens INTEGER,
		cache_reported INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER,
		total_tokens INTEGER,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at TEXT
	)`)
	if err != nil {
		return fmt.Errorf("create usage runs table: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS usage_runs_session_index
		ON usage_runs (user_id, session_id, status)`)
	if err != nil {
		return fmt.Errorf("create usage runs session index: %w", err)
	}
	return nil
}

func (s *store) start(ctx context.Context, run Run) error {
	if run.RequestID == "" || run.UserID == "" || run.SessionID == "" || run.ModelID == "" {
		return errors.New("requestID, userID, sessionID, and modelID are required")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO usage_runs (
		request_id, user_id, session_id, model_id, status
	) VALUES (?, ?, ?, ?, ?) ON CONFLICT(request_id) DO NOTHING`,
		run.RequestID, run.UserID, run.SessionID, run.ModelID, runningStatus,
	)
	if err != nil {
		return fmt.Errorf("start agent run %q: %w", run.RequestID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return ErrRunAlreadyExists
	}
	return nil
}

func (s *store) finish(ctx context.Context, run Run, tokens Tokens) error {
	cacheReported := tokens.CachedPromptTokens != nil
	var cached any
	if cacheReported {
		cached = *tokens.CachedPromptTokens
	}
	result, err := s.db.ExecContext(ctx, `UPDATE usage_runs
		SET prompt_tokens = ?, cached_prompt_tokens = ?, cache_reported = ?,
			completion_tokens = ?, total_tokens = ?, status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE request_id = ? AND status = ?`,
		tokens.PromptTokens, cached, cacheReported, tokens.CompletionTokens,
		tokens.TotalTokens, completedStatus, run.RequestID, runningStatus,
	)
	if err != nil {
		return fmt.Errorf("finish agent run %q: %w", run.RequestID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return fmt.Errorf("finish agent run %q: %w", run.RequestID, ErrRunNotFound)
	}
	return nil
}

func (s *store) fail(ctx context.Context, requestID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE usage_runs
		SET status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE request_id = ? AND status = ?`, failedStatus, requestID, runningStatus)
	return err
}

func (s *store) status(ctx context.Context, userID, requestID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM usage_runs
		WHERE user_id = ? AND request_id = ?`, userID, requestID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRunNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read agent run status: %w", err)
	}
	return status, nil
}

func (s *store) sessionSummary(ctx context.Context, userID, sessionID string) (Summary, error) {
	var row struct {
		count      int
		total      int
		completion int
		prompt     int
		cacheKnown int
		cached     sql.NullInt64
	}
	err := s.db.QueryRowContext(ctx, `SELECT
		COUNT(*), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(cache_reported), 0), SUM(cached_prompt_tokens)
		FROM usage_runs WHERE user_id = ? AND session_id = ? AND status = ?`,
		userID, sessionID, completedStatus,
	).Scan(&row.count, &row.total, &row.completion, &row.prompt, &row.cacheKnown, &row.cached)
	if err != nil {
		return Summary{}, fmt.Errorf("summarize session usage: %w", err)
	}

	summary := Summary{TotalTokens: row.total, CompletionTokens: row.completion}
	if row.count == 0 || row.cacheKnown != row.count || !row.cached.Valid {
		return summary, nil
	}
	cached := int(row.cached.Int64)
	uncached := row.prompt - cached
	cacheHitRate := 0.0
	if row.prompt > 0 {
		cacheHitRate = float64(cached) / float64(row.prompt)
	}
	summary.CachedPromptTokens = &cached
	summary.UncachedPromptTokens = &uncached
	summary.CacheHitRate = &cacheHitRate
	return summary, nil
}
