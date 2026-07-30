// Package webapi exposes the private HTTP API consumed by EDITH's Web BFF.
package webapi

import (
	"database/sql"
	"net/http"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// Server owns the long-lived capabilities needed by Web BFF handlers.
// Per-request user, session, message, RunOptions, and timeline state stay in
// the handler that uses them.
type Server struct {
	AppName  string
	Runner   runner.Runner
	Users    *userconfig.Store
	Images   *images.Service
	Sessions session.Service
	UsageDB  *sql.DB
}

// Register attaches Web BFF routes to mux.
func (s Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/agent-runs", s.runAgent)
	mux.HandleFunc("GET /internal/models", s.listModels)
	mux.HandleFunc("GET /internal/available-models", s.listAvailableModels)
	mux.HandleFunc("GET /internal/user-settings", s.getUserSettings)
	mux.HandleFunc("PUT /internal/user-settings", s.saveUserSettings)
	mux.HandleFunc("GET /internal/mcp-servers", s.listMCPServers)
	mux.HandleFunc("POST /internal/mcp-servers", s.createMCPServer)
	mux.HandleFunc("PUT /internal/mcp-servers/{serverID}", s.updateMCPServer)
	mux.HandleFunc("DELETE /internal/mcp-servers/{serverID}", s.deleteMCPServer)
	mux.HandleFunc("GET /internal/conversations", s.listConversations)
	mux.HandleFunc("GET /internal/conversations/{sessionID}", s.getConversation)
	mux.HandleFunc("POST /internal/images", s.createImageUpload)
	mux.HandleFunc("POST /internal/images/{imageID}/complete", s.completeImageUpload)
	mux.HandleFunc("GET /internal/images/{imageID}", s.openImage)
}
