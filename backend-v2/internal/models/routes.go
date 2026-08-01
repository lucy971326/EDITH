package models

import "net/http"

// Register 注册模型目录路由；由 main 显式调用。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/models", h.listCatalog)
	mux.HandleFunc("GET /internal/available-models", h.listAvailable)
}
