package userconfig

import (
	"context"
	"database/sql"
	"fmt"
)

// createSchema 创建本模块拥有的所有表。
func createSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_agents (
			user_id TEXT PRIMARY KEY,
			personality TEXT NOT NULL DEFAULT '',
			default_model_id TEXT NOT NULL DEFAULT '',
			timezone TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS user_providers (
			user_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			api_key TEXT NOT NULL,
			PRIMARY KEY (user_id, provider_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_mcp_servers (
			server_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			transport TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_mcp_headers (
			server_id TEXT NOT NULL,
			header_name TEXT NOT NULL,
			header_value TEXT NOT NULL,
			PRIMARY KEY (server_id, header_name)
		)`,
		`CREATE TABLE IF NOT EXISTS user_channel_bindings (
			channel TEXT NOT NULL,
			external_user_id TEXT NOT NULL,
			clerk_user_id TEXT NOT NULL,
			PRIMARY KEY (channel, external_user_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(context.Background(), statement); err != nil {
			return fmt.Errorf("create userconfig schema: %w", err)
		}
	}
	return nil
}
