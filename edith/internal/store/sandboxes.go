package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SessionKey struct {
	AppName   string
	UserID    string
	SessionID string
}

// FindSandboxID 查不到返回 ("", false, nil)；数据库异常才返回 error。
func (s *Store) FindSandboxID(ctx context.Context, key SessionKey) (string, bool, error) {
	var sbxID string
	err := s.db.QueryRowContext(ctx,
		`SELECT sandbox_id FROM session_sandboxes
		 WHERE app_name = ? AND user_id = ? AND session_id = ?`,
		key.AppName, key.UserID, key.SessionID,
	).Scan(&sbxID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: find sandbox %v: %w", key, err)
	}
	return sbxID, true, nil
}

// SaveSandbox 按 Session Key 保存。
func (s *Store) SaveSandbox(ctx context.Context, key SessionKey, sandboxID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO session_sandboxes (app_name, user_id, session_id, sandbox_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(app_name, user_id, session_id) DO UPDATE SET sandbox_id = excluded.sandbox_id`,
		key.AppName, key.UserID, key.SessionID, sandboxID,
	)
	if err != nil {
		return fmt.Errorf("store: save sandbox %v = %s: %w", key, sandboxID, err)
	}
	return nil
}