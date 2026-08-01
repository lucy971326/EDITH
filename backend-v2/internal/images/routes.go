package images

import "net/http"

// Register 把图片模块的 HTTP 路由注册到主服务 mux。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/images", h.CreateUpload)
	mux.HandleFunc("POST /internal/images/{imageID}/complete", h.CompleteUpload)
	mux.HandleFunc("GET /internal/images/{imageID}", h.OpenImage)
}
