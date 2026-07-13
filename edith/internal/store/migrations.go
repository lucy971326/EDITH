package store

import (
	"context"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		clerk_user_id   TEXT PRIMARY KEY,
		github_user_id  INTEGER NOT NULL UNIQUE,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS github_installations (
		installation_id INTEGER PRIMARY KEY,
		clerk_user_id   TEXT NOT NULL UNIQUE,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (clerk_user_id) REFERENCES users(clerk_user_id)
	)`,
	`CREATE TABLE IF NOT EXISTS session_sandboxes (
		app_name    TEXT NOT NULL,
		user_id     TEXT NOT NULL,
		session_id  TEXT NOT NULL,
		sandbox_id  TEXT NOT NULL UNIQUE,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (app_name, user_id, session_id)
	)`,
	`CREATE TABLE IF NOT EXISTS webhook_deliveries (
		delivery_id TEXT PRIMARY KEY,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
}

// Migrate 顺序执行所有 DDL；IF NOT EXISTS 保证幂等。
func (s *Store) Migrate(ctx context.Context) error {
	for i, ddl := range migrations {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("store: migration[%d]: %w | ddl: %s", i, err, ddl)
		}
	}
	return nil
}
