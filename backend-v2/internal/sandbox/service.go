package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"edith/backend-v2/internal/volume"
	"github.com/eric642/e2b-go-sdk"
)

// service 负责把用户会话绑定到唯一 E2B Sandbox；仅供本模块工具使用。
type service struct {
	db       *sql.DB
	client   *e2b.Client
	template string
	volumes  *volume.Service
}

var errSandboxNotFound = errors.New("sandbox mapping not found")

// ExistingWorkspace 仅连接已经由 Agent 工具创建并保存的 Sandbox。
// 它不会创建 Sandbox、挂载或 Volume，也不会更新本地映射，供只读 HTTP 使用。
func (s *service) ExistingWorkspace(ctx context.Context, userID, sessionID string) (*e2b.Sandbox, error) {
	userID, sessionID = strings.TrimSpace(userID), strings.TrimSpace(sessionID)
	if userID == "" || sessionID == "" {
		return nil, errors.New("sandbox requires a user ID and session ID")
	}
	var sandboxID string
	err := s.db.QueryRowContext(ctx, `SELECT sandbox_id FROM user_sandboxes WHERE user_id = ? AND session_id = ?`, userID, sessionID).Scan(&sandboxID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSandboxNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load sandbox: %w", err)
	}
	workspace, err := s.client.Connect(ctx, sandboxID, e2b.ConnectOptions{Timeout: connectTimeout})
	if err != nil {
		return nil, fmt.Errorf("connect sandbox: %w", err)
	}
	return workspace, nil
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
	mount, err := s.volumes.MountForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("prepare user volume: %w", err)
	}
	workspace, err := s.client.Create(ctx, e2b.CreateOptions{Template: s.template, Timeout: connectTimeout, Secure: true, Metadata: map[string]string{"edith_user_id": userID, "edith_session_id": sessionID}, Lifecycle: &e2b.LifecycleOptions{OnTimeout: "pause", AutoResume: true}, VolumeMounts: []e2b.VolumeMount{{Name: mount.Name, Path: mount.Path}}})
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO user_sandboxes (user_id, session_id, sandbox_id) VALUES (?, ?, ?)`, userID, sessionID, workspace.ID); err != nil {
		return nil, fmt.Errorf("save sandbox mapping: %w", err)
	}
	return workspace, nil
}
