package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleRunStatus 查询一个活跃任务。
// 输入：BFF 注入的可信 userId 与路径中的 requestId。
// 输出：任务仍由 ManagedRunner 管理时返回 running；不存在或不属于该用户时返回 404。
func (s *Server) handleRunStatus(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if userID == "" || requestID == "" {
		http.Error(w, "userId and requestId are required", http.StatusBadRequest)
		return
	}

	status, ok := s.runner.RunStatus(requestID)
	if !ok || status.SessionKey.UserID != userID {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(RunStatusResponse{RequestID: requestID, Status: "running"})
}
