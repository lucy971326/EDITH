package conversation

import (
	"errors"
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
)

// HTTP 是会话模块对 Web BFF 提供的 HTTP 能力。
type HTTP struct{ history *history }

// ListConversations 返回用户的会话侧边栏摘要。
func (h *HTTP) ListConversations(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	conversations, err := h.history.List(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "conversation_list_failed", "cannot list conversations")
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, ConversationListResponse{Conversations: conversations})
}

// GetConversation 返回指定会话的 Timeline 和累计用量。
func (h *HTTP) GetConversation(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	sessionID := strings.TrimSpace(request.PathValue("sessionID"))
	if userID == "" || sessionID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and sessionID are required")
		return
	}
	conversation, err := h.history.Get(request.Context(), userID, sessionID)
	if errors.Is(err, errConversationNotFound) {
		httpx.WriteError(writer, http.StatusNotFound, "conversation_not_found", "conversation not found")
		return
	}
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "conversation_read_failed", "cannot read conversation")
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, conversation)
}
