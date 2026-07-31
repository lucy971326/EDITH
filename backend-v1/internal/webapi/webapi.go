// Package webapi exposes the private HTTP API consumed by EDITH's Web BFF.
package webapi

import (
	"database/sql"
	"net/http"

	"edith/backend-v1/internal/cronjob"
	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/session"
)

// Server owns the long-lived capabilities needed by Web BFF handlers.
// Per-request user, session, message, RunOptions, and timeline state stay in
// the handler that uses them.
type Server struct {
	AppName  string
	Users    *userconfig.Store
	CronJobs *cronjob.Store
	Images   *images.Service
	Sessions session.Service
	UsageDB  *sql.DB
}

// Register attaches Web BFF routes to mux.
func (s Server) Register(mux *http.ServeMux) {
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
	mux.HandleFunc("GET /internal/cron-jobs", s.listCronJobs)
	mux.HandleFunc("POST /internal/cron-jobs", s.createCronJob)
	mux.HandleFunc("PUT /internal/cron-jobs/{jobID}", s.updateCronJob)
	mux.HandleFunc("DELETE /internal/cron-jobs/{jobID}", s.deleteCronJob)
	mux.HandleFunc("POST /internal/cron-jobs/{jobID}/toggle", s.toggleCronJob)
}
