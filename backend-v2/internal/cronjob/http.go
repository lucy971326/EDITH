package cronjob

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"edith/backend-v2/internal/httpx"
	"edith/backend-v2/internal/userconfig"
)

// HTTP 是 cronjob 的 Web 管理入口。
type HTTP struct {
	jobs     *store
	settings *userconfig.Settings
}

func (h *HTTP) listJobs(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	jobs, err := h.jobs.List(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cronjob_error", err.Error())
		return
	}
	responses := make([]JobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, jobResponse(job))
	}
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Jobs: responses})
}
func (h *HTTP) createJob(w http.ResponseWriter, r *http.Request) {
	var input CreateRequest
	if err := httpx.ReadJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if timezone := strings.TrimSpace(input.Timezone); timezone != "" {
		if err := h.settings.SaveTimezone(r.Context(), input.UserID, timezone); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "cronjob_error", err.Error())
			return
		}
	}
	job, err := h.jobs.Create(r.Context(), input.UserID, JobInput{Name: input.Name, TaskType: input.TaskType, Schedule: input.Schedule, Prompt: input.Prompt})
	if err != nil {
		h.writeJobError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, jobResponse(job))
}
func (h *HTTP) updateJob(w http.ResponseWriter, r *http.Request) {
	var input UpdateRequest
	if err := httpx.ReadJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	job, err := h.jobs.Update(r.Context(), input.UserID, r.PathValue("jobID"), JobInput{Name: input.Name, TaskType: input.TaskType, Schedule: input.Schedule, Prompt: input.Prompt})
	if err != nil {
		h.writeJobError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, jobResponse(job))
}
func (h *HTTP) deleteJob(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if err := h.jobs.Delete(r.Context(), userID, r.PathValue("jobID")); err != nil {
		h.writeJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *HTTP) toggleJob(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}
	enabled := strings.TrimSpace(r.URL.Query().Get("enabled")) == "true"
	job, err := h.jobs.SetEnabled(r.Context(), userID, r.PathValue("jobID"), enabled)
	if err != nil {
		h.writeJobError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, jobResponse(job))
}
func (h *HTTP) writeJobError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "cron_job_not_found", "定时任务不存在")
		return
	}
	if errors.Is(err, ErrInvalidJob) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_cron_job", err.Error())
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, "cronjob_error", err.Error())
}
func jobResponse(job Job) JobResponse {
	var next *string
	if job.NextRunAt != nil {
		value := job.NextRunAt.Format(time.RFC3339)
		next = &value
	}
	return JobResponse{ID: job.ID, Name: job.Name, TaskType: job.TaskType, Schedule: job.Schedule, Prompt: job.Prompt, Enabled: job.Enabled, NextRunAt: next, Running: job.Running, CreatedAt: job.CreatedAt.Format(time.RFC3339)}
}
