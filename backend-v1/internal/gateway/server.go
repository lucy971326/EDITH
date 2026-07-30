package gateway

import (
	"database/sql"
	"errors"
	"net/http"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Server owns EDITH's long-lived Agent execution capabilities. Per-message
// data stays in MessageRequest and local variables in run.go.
type Server struct {
	runner   runner.ManagedRunner
	users    *userconfig.Store
	images   *images.Service
	usageDB  *sql.DB
	lanes    *sessionLanes
	canceled *cancelTracker
}

func New(
	runner runner.ManagedRunner,
	users *userconfig.Store,
	images *images.Service,
	usageDB *sql.DB,
) (*Server, error) {
	if runner == nil {
		return nil, errors.New("gateway runner is required")
	}
	if users == nil {
		return nil, errors.New("gateway user config store is required")
	}
	if images == nil {
		return nil, errors.New("gateway image service is required")
	}
	if usageDB == nil {
		return nil, errors.New("gateway usage database is required")
	}
	return &Server{
		runner:   runner,
		users:    users,
		images:   images,
		usageDB:  usageDB,
		lanes:    newSessionLanes(),
		canceled: newCancelTracker(),
	}, nil
}

// Register attaches the Agent Gateway HTTP protocol to the application mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/gateway/messages:stream", s.handleStreamMessage)
	mux.HandleFunc("GET /internal/gateway/runs/{requestID}", s.handleRunStatus)
	mux.HandleFunc("POST /internal/gateway/runs/{requestID}/cancel", s.handleCancel)
}
