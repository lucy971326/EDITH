package gateway

import (
	"net/http"
	"strings"
)

// handleCancel 停止一个活跃任务。
// 输入：BFF 注入的可信 userId 与路径中的 requestId。
// 输出：ManagedRunner 接受取消信号时返回 204；任务不存在或不属于该用户时返回 404。
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
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
	if !s.runner.Cancel(requestID) {
		http.Error(w, "agent run not found", http.StatusNotFound)
		return
	}

	s.userCancelMarks.mark(requestID)
	w.WriteHeader(http.StatusNoContent)
}
