package volume

import (
	"context"
	"database/sql"
	"fmt"
)

type store struct {
	db *sql.DB
}

// createSchema 创建 Volume 模块独占的用户映射表。
func createSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS user_volumes (
		user_id TEXT NOT NULL PRIMARY KEY,
		volume_id TEXT NOT NULL UNIQUE,
		volume_name TEXT NOT NULL UNIQUE,
		volume_token TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create user_volumes table: %w", err)
	}
	return nil
}

// load 按用户读取已保存的 Volume 映射。
func (s *store) load(ctx context.Context, userID string) (record, error) {
	var result record
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, volume_id, volume_name, volume_token
		FROM user_volumes WHERE user_id = ?
	`, userID).Scan(&result.UserID, &result.ID, &result.Name, &result.Token)
	if err != nil {
		return record{}, err
	}
	return result, nil
}

// save 保存新建的用户 Volume 映射。
func (s *store) save(ctx context.Context, value record) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_volumes (user_id, volume_id, volume_name, volume_token)
		VALUES (?, ?, ?, ?)
	`, value.UserID, value.ID, value.Name, value.Token)
	if err != nil {
		return fmt.Errorf("save user volume: %w", err)
	}
	return nil
}
