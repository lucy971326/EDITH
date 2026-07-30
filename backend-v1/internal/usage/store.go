package usage

import (
	"context"
	"database/sql"
	"fmt"
)

func createTable(db *sql.DB, ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS agent_runs (
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
		return fmt.Errorf("create agent runs table: %w", err)
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS agent_runs_session_index
		ON agent_runs (user_id, session_id, status)`)
	if err != nil {
		return fmt.Errorf("create agent runs session index: %w", err)
	}
	return nil
}
