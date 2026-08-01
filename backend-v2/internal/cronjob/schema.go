package cronjob

import (
	"context"
	"database/sql"
	"fmt"
)

// createSchema 创建 cronjob 模块独占的数据表。
func createSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cron_jobs (
			id TEXT PRIMARY KEY,
			clerk_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			task_type TEXT NOT NULL,
			schedule TEXT NOT NULL,
			prompt TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			next_run_at TEXT,
			running INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create cron_jobs table: %w", err)
	}
	return nil
}
