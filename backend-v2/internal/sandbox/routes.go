package sandbox

import "net/http"

// Register 把 Sandbox 只读文件接口注册到主服务 mux。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/sandbox/files", h.listFiles)
	mux.HandleFunc("GET /internal/sandbox/files/content", h.readContent)
}
