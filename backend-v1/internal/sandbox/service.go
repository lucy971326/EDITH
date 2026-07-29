// Package sandbox manages EDITH's persistent E2B workspaces.
package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eric642/e2b-go-sdk"
	_ "github.com/mattn/go-sqlite3"
)

const sandboxTimeout = 10 * time.Minute

// Service owns EDITH's E2B client and the user/session-to-sandbox mapping.
// It is a long-lived capability created once in main.
type Service struct {
	db       *sql.DB
	client   *e2b.Client
	template string
}

// Open creates the mapping table and one reusable E2B client.
func Open(databasePath string, template string) (*Service, error) {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil, errors.New("sandbox template is required")
	}

	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sandbox database: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_sandboxes (
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		sandbox_id TEXT NOT NULL UNIQUE,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, session_id)
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create user sandboxes table: %w", err)
	}

	client, err := e2b.NewClient(e2b.Config{})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create E2B client: %w", err)
	}

	return &Service{db: db, client: client, template: template}, nil
}

// Close releases the SQLite connection. E2B sandbox lifecycle is managed by
// its auto-pause policy, so there is no sandbox to close here.
func (s *Service) Close() error {
	return s.db.Close()
}

// OpenWorkspace returns the E2B sandbox for one user session. It creates the
// sandbox on first use; later calls reconnect and automatically resume it when
// E2B has paused it after inactivity.
func (s *Service) OpenWorkspace(ctx context.Context, userID, sessionID string) (*e2b.Sandbox, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if userID == "" || sessionID == "" {
		return nil, errors.New("sandbox requires userID and sessionID")
	}

	var sandboxID string
	err := s.db.QueryRowContext(ctx, `
		SELECT sandbox_id
		FROM user_sandboxes
		WHERE user_id = ? AND session_id = ?
	`, userID, sessionID).Scan(&sandboxID)
	if err == nil {
		workspace, err := s.client.Connect(ctx, sandboxID, e2b.ConnectOptions{
			Timeout: sandboxTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("connect sandbox for %q/%q: %w", userID, sessionID, err)
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE user_sandboxes
			SET updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND session_id = ?
		`, userID, sessionID); err != nil {
			return nil, fmt.Errorf("update sandbox use time for %q/%q: %w", userID, sessionID, err)
		}
		return workspace, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load sandbox for %q/%q: %w", userID, sessionID, err)
	}

	workspace, err := s.client.Create(ctx, e2b.CreateOptions{
		Template: s.template,
		Timeout:  sandboxTimeout,
		Secure:   true,
		Metadata: map[string]string{
			"edith_user_id":    userID,
			"edith_session_id": sessionID,
		},
		Lifecycle: &e2b.LifecycleOptions{
			OnTimeout:  "pause",
			AutoResume: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create sandbox for %q/%q: %w", userID, sessionID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_sandboxes (user_id, session_id, sandbox_id)
		VALUES (?, ?, ?)
	`, userID, sessionID, workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("save sandbox for %q/%q: %w", userID, sessionID, err)
	}

	return workspace, nil
}
