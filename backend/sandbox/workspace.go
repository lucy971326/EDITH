package sandbox

import (
	"context"
	"errors"
)

// WorkspaceID identifies one user's isolated session workspace.
type WorkspaceID struct {
	UserID    string
	SessionID string
}

func (id WorkspaceID) validate() error {
	if id.UserID == "" {
		return errors.New("workspace user ID is required")
	}
	if id.SessionID == "" {
		return errors.New("workspace session ID is required")
	}
	return nil
}

// BackendProvider returns the execution backend for a workspace.
// It may reuse an existing backend or lazily create one.
type BackendProvider interface {
	GetBackend(ctx context.Context, id WorkspaceID) (ExecBackend, error)
	LoadUserSkillsOverview(ctx context.Context, userID string) (string, error)
	Close() error
}
