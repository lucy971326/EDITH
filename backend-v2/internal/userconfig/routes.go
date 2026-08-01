package userconfig

import "net/http"

// Register 注册用户设置与 MCP 的 HTTP 路由；由 main 显式调用。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/user-settings", h.getSettings)
	mux.HandleFunc("PUT /internal/user-settings", h.saveSettings)
	mux.HandleFunc("GET /internal/mcp-servers", h.listMCPServers)
	mux.HandleFunc("POST /internal/mcp-servers", h.createMCPServer)
	mux.HandleFunc("PUT /internal/mcp-servers/{serverID}", h.updateMCPServer)
	mux.HandleFunc("DELETE /internal/mcp-servers/{serverID}", h.deleteMCPServer)
}
