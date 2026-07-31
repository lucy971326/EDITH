package webadapter

import (
	"encoding/json"
	"net/http"

	"edith/backend-v1/internal/gateway"
)

// handleRunStatus 将 Web 的任务状态查询转换为 Gateway 调用。
func (s *Server) handleRunStatus(w http.ResponseWriter, r *http.Request) {
	status, apiError := s.agentGateway.RunStatus(r.URL.Query().Get("userId"), r.PathValue("requestID"))
	if apiError != nil {
		writeGatewayError(w, apiError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// handleCancel 将 Web 的主动停止请求转换为 Gateway 调用。
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	apiError := s.agentGateway.Cancel(r.URL.Query().Get("userId"), r.PathValue("requestID"))
	if apiError != nil {
		writeGatewayError(w, apiError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeGatewayError(w http.ResponseWriter, apiError *gateway.APIError) {
	status := http.StatusInternalServerError
	switch apiError.Type {
	case "invalid_request":
		status = http.StatusBadRequest
	case "session_busy", "request_conflict":
		status = http.StatusConflict
	case "not_found":
		status = http.StatusNotFound
	}
	writeError(w, status, *apiError)
}

func writeError(w http.ResponseWriter, status int, apiError gateway.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError)
}
