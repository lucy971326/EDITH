package conversation

import "net/http"

// Register 把会话模块的 HTTP 路由注册到主服务 mux。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/conversations", h.ListConversations)
	mux.HandleFunc("GET /internal/conversations/{sessionID}", h.GetConversation)
}
