package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/eric642/e2b-go-sdk"
)

// service 负责把用户会话绑定到唯一 E2B Sandbox；仅供本模块工具使用。
type service struct {
	db       *sql.DB
	client   *e2b.Client
	template string
}

// Workspace 为当前用户会话返回可恢复的 E2B Sandbox；首次调用才会创建远端资源。
func (s *service) Workspace(ctx context.Context, userID, sessionID string) (*e2b.Sandbox, error) {
	userID, sessionID = strings.TrimSpace(userID), strings.TrimSpace(sessionID)
	if userID == "" || sessionID == "" {
		return nil, errors.New("sandbox requires a user ID and session ID")
	}
	var sandboxID string
	err := s.db.QueryRowContext(ctx, `SELECT sandbox_id FROM user_sandboxes WHERE user_id = ? AND session_id = ?`, userID, sessionID).Scan(&sandboxID)
	if err == nil {
		workspace, err := s.client.Connect(ctx, sandboxID, e2b.ConnectOptions{Timeout: connectTimeout})
		if err != nil {
			return nil, fmt.Errorf("connect sandbox: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE user_sandboxes SET updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND session_id = ?`, userID, sessionID); err != nil {
			return nil, fmt.Errorf("update sandbox access: %w", err)
		}
		return workspace, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load sandbox: %w", err)
	}
	workspace, err := s.client.Create(ctx, e2b.CreateOptions{Template: s.template, Timeout: connectTimeout, Secure: true, Metadata: map[string]string{"edith_user_id": userID, "edith_session_id": sessionID}, Lifecycle: &e2b.LifecycleOptions{OnTimeout: "pause", AutoResume: true}})
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO user_sandboxes (user_id, session_id, sandbox_id) VALUES (?, ?, ?)`, userID, sessionID, workspace.ID); err != nil {
		return nil, fmt.Errorf("save sandbox mapping: %w", err)
	}
	return workspace, nil
}
