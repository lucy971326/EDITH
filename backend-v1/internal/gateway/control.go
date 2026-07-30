package gateway

import (
	"net/http"
	"strings"
)

func (s *Server) RunStatus(userID, requestID string) (RunStatusResponse, bool) {
	status, ok := s.runner.RunStatus(strings.TrimSpace(requestID))
	if !ok || status.SessionKey.UserID != strings.TrimSpace(userID) {
		return RunStatusResponse{}, false
	}
	return RunStatusResponse{RequestID: requestID, Status: "running"}, true
}

func (s *Server) Cancel(userID, requestID string) bool {
	if _, ok := s.RunStatus(userID, requestID); !ok {
		return false
	}
	canceled := s.runner.Cancel(strings.TrimSpace(requestID))
	if canceled {
		s.canceled.mark(strings.TrimSpace(requestID))
	}
	return canceled
}

func (s *Server) handleRunStatus(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if userID == "" || requestID == "" {
		http.Error(w, "userId and requestId are required", http.StatusBadRequest)
		return
	}
	status, ok := s.RunStatus(userID, requestID)
	if !ok {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if userID == "" || requestID == "" {
		http.Error(w, "userId and requestId are required", http.StatusBadRequest)
		return
	}
	if !s.Cancel(userID, requestID) {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
