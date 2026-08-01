package sandbox

import (
	"context"
	"database/sql"
	"fmt"
)

// createSchema 创建 Sandbox 模块独占的用户会话映射表。
func createSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS user_sandboxes (
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		sandbox_id TEXT NOT NULL UNIQUE,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, session_id)
	)`)
	if err != nil {
		return fmt.Errorf("create user_sandboxes table: %w", err)
	}
	return nil
}
