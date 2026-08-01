package cronjob

import "net/http"

// Register 把 cronjob 自己拥有的 HTTP 路由注册到主 mux。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/cron-jobs", h.listJobs)
	mux.HandleFunc("POST /internal/cron-jobs", h.createJob)
	mux.HandleFunc("PUT /internal/cron-jobs/{jobID}", h.updateJob)
	mux.HandleFunc("DELETE /internal/cron-jobs/{jobID}", h.deleteJob)
	mux.HandleFunc("POST /internal/cron-jobs/{jobID}/toggle", h.toggleJob)
}
