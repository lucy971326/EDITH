package webapi

import (
	"log"
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/runner"

	"github.com/google/uuid"
)

// getAgentRunStatus asks the ManagedRunner whether one Agent Run is still
// executing. ManagedRunner is the requestID control plane; the usage ledger
// is not involved in run control.
func (s Server) getAgentRunStatus(w http.ResponseWriter, r *http.Request) {
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

	mr, ok := s.Runner.(runner.ManagedRunner)
	if !ok {
		http.Error(w, "runner does not support run control", http.StatusInternalServerError)
		return
	}

	status, ok := mr.RunStatus(requestID)
	if !ok {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}
	if status.SessionKey.UserID != userID {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}

	writeJSON(w, AgentRunStatusResponse{RequestID: requestID, Status: AgentRunStatusRunning})
}

// cancelAgentRun cancels a running Agent Run by requestID. It verifies the
// caller owns the run through the Runner's live session key, not the ledger.
func (s Server) cancelAgentRun(w http.ResponseWriter, r *http.Request) {
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

	mr, ok := s.Runner.(runner.ManagedRunner)
	if !ok {
		http.Error(w, "runner does not support run control", http.StatusInternalServerError)
		return
	}

	status, ok := mr.RunStatus(requestID)
	if !ok {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}
	if status.SessionKey.UserID != userID {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}

	cancelled := mr.Cancel(requestID)
	log.Printf("cancel agent run %q: managed runner found=%v", requestID, cancelled)
	w.WriteHeader(http.StatusNoContent)
}
