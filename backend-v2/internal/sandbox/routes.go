package sandbox

import "net/http"

// Register 把 Sandbox 文件接口注册到主服务 mux。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/sandbox/files", h.listFiles)
	mux.HandleFunc("GET /internal/sandbox/files/content", h.readContent)
	mux.HandleFunc("POST /internal/sandbox/files/upload", h.uploadFile)
	mux.HandleFunc("GET /internal/sandbox/files/download", h.downloadFile)
}
