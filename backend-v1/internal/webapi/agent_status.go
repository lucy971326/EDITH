package webapi

import (
	"errors"
	"net/http"
	"strings"

	"edith/backend-v1/internal/usage"

	"github.com/google/uuid"
)

func (s Server) getAgentRunStatus(w http.ResponseWriter, r *http.Request) {
	if s.UsageDB == nil {
		http.Error(w, "usage service is unavailable", http.StatusServiceUnavailable)
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(requestID); err != nil {
		http.Error(w, "requestId must be a UUID", http.StatusBadRequest)
		return
	}

	status, err := usage.Status(s.UsageDB, r.Context(), userID, requestID)
	if errors.Is(err, usage.ErrRunNotFound) {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "load agent run status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, AgentRunStatusResponse{RequestID: requestID, Status: status})
}
