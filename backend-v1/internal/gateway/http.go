package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleStreamMessage(w http.ResponseWriter, r *http.Request) {
	var request MessageRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid gateway message request", http.StatusBadRequest)
		return
	}

	// The HTTP connection only watches the task. The task itself must survive
	// a browser disconnect and is stopped through ManagedRunner.Cancel instead.
	taskContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, apiErr, status := s.StreamMessage(taskContext, request)
	if apiErr != nil {
		writeJSONStatus(w, apiErr, status)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	clientConnected := true
	for event := range events {
		if clientConnected && writeSSE(w, event) != nil {
			clientConnected = false
		}
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONStatus(w http.ResponseWriter, value any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeSSE(w http.ResponseWriter, event StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode gateway SSE event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("write gateway SSE event: %w", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
