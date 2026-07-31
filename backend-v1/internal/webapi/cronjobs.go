package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"edith/backend-v1/internal/cronjob"
)

func (s Server) listCronJobs(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}
	jobs, err := s.CronJobs.List(r.Context(), userID)
	if err != nil {
		http.Error(w, "list cron jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, CronJobListResponse{Jobs: cronJobResponses(jobs)})
}

func (s Server) createCronJob(w http.ResponseWriter, r *http.Request) {
	request, err := decodeCronJobRequest(w, r)
	if err != nil {
		return
	}
	if timezone := strings.TrimSpace(request.Timezone); timezone != "" {
		if err := s.Users.SaveTimezone(r.Context(), request.UserID, timezone); err != nil {
			http.Error(w, "save user timezone: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	job, err := s.CronJobs.Create(r.Context(), request.UserID, cronJobInput(request))
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, cronJobResponse(job))
}

func (s Server) updateCronJob(w http.ResponseWriter, r *http.Request) {
	request, err := decodeCronJobRequest(w, r)
	if err != nil {
		return
	}
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	if jobID == "" {
		http.Error(w, "jobID is required", http.StatusBadRequest)
		return
	}
	job, err := s.CronJobs.Update(r.Context(), request.UserID, jobID, cronJobInput(request))
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	writeJSON(w, cronJobResponse(job))
}

func (s Server) deleteCronJob(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	if userID == "" || jobID == "" {
		http.Error(w, "userId and jobID are required", http.StatusBadRequest)
		return
	}
	if err := s.CronJobs.Delete(r.Context(), userID, jobID); err != nil {
		writeCronJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Server) toggleCronJob(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	enabled := strings.TrimSpace(r.URL.Query().Get("enabled")) == "true"
	if userID == "" || jobID == "" {
		http.Error(w, "userId and jobID are required", http.StatusBadRequest)
		return
	}
	job, err := s.CronJobs.SetEnabled(r.Context(), userID, jobID, enabled)
	if err != nil {
		writeCronJobError(w, err)
		return
	}
	writeJSON(w, cronJobResponse(job))
}

func decodeCronJobRequest(w http.ResponseWriter, r *http.Request) (CronJobRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request CronJobRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid cron job request", http.StatusBadRequest)
		return CronJobRequest{}, err
	}
	request.UserID = strings.TrimSpace(request.UserID)
	request.Name = strings.TrimSpace(request.Name)
	request.Schedule = strings.TrimSpace(request.Schedule)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Timezone = strings.TrimSpace(request.Timezone)
	if request.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return CronJobRequest{}, errors.New("missing userId")
	}
	return request, nil
}

func cronJobInput(request CronJobRequest) cronjob.JobInput {
	return cronjob.JobInput{
		Name:     request.Name,
		TaskType: request.TaskType,
		Schedule: request.Schedule,
		Prompt:   request.Prompt,
	}
}

func cronJobResponses(jobs []cronjob.Job) []CronJobResponse {
	responses := make([]CronJobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, cronJobResponse(job))
	}
	return responses
}

func cronJobResponse(job cronjob.Job) CronJobResponse {
	response := CronJobResponse{
		ID:        job.ID,
		Name:      job.Name,
		TaskType:  job.TaskType,
		Schedule:  job.Schedule,
		Prompt:    job.Prompt,
		Enabled:   job.Enabled,
		Running:   job.Running,
		CreatedAt: job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if job.NextRunAt != nil {
		formatted := job.NextRunAt.Format("2006-01-02T15:04:05Z07:00")
		response.NextRunAt = &formatted
	}
	return response
}

func writeCronJobError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cronjob.ErrInvalidJob):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, cronjob.ErrNotFound):
		http.Error(w, "cron job not found", http.StatusNotFound)
	default:
		http.Error(w, "cron job operation: "+err.Error(), http.StatusInternalServerError)
	}
}
