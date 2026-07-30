package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/mcp"
	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/runopts"
	"edith/backend-v1/internal/timeline"
	"edith/backend-v1/internal/usage"

	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func (s Server) runAgent(w http.ResponseWriter, r *http.Request) {
	request, err := decodeAgentRunRequest(w, r)
	if err != nil {
		return
	}
	if s.Usage == nil {
		http.Error(w, "usage service is unavailable", http.StatusServiceUnavailable)
		return
	}

	definition, ok := models.Lookup(request.ModelID)
	if !ok {
		http.Error(w, "unsupported modelId", http.StatusBadRequest)
		return
	}
	if len(request.ImageIDs) > 0 && !definition.Info.Capabilities.Vision {
		http.Error(w, "selected model does not support image input", http.StatusBadRequest)
		return
	}
	apiKey, err := s.Users.LoadProviderAPIKey(r.Context(), request.UserID, definition.ProviderID)
	if err != nil {
		http.Error(w, "load model credential: "+err.Error(), http.StatusBadRequest)
		return
	}
	personality, err := s.Users.LoadPersonality(r.Context(), request.UserID)
	if err != nil {
		http.Error(w, "load user personality: "+err.Error(), http.StatusBadRequest)
		return
	}
	message := model.NewUserMessage(request.Message)
	runContext := images.WithHydratedSession(r.Context())
	if len(request.ImageIDs) > 0 {
		if s.Images == nil {
			http.Error(w, "image service is unavailable", http.StatusServiceUnavailable)
			return
		}
		runContext, err = s.Images.AddMessageImages(
			runContext,
			request.UserID,
			request.SessionID,
			request.ImageIDs,
			&message,
		)
		if err != nil {
			http.Error(w, "prepare message images: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	mcpServers, err := s.Users.LoadEnabledMCPServers(r.Context(), request.UserID)
	if err != nil {
		http.Error(w, "load MCP servers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mcpTools, closeMCP, err := mcp.OpenTools(r.Context(), mcpServers)
	if err != nil {
		http.Error(w, "open MCP tools: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = closeMCP() }()

	requestID := uuid.NewString()
	opts := runopts.Build(runopts.Config{
		RequestID: requestID,
		Stream:    true,
		ModelName: request.ModelID,
		APIKey:    apiKey,
		GlobalInstruction: "你是 EDITH AI Agent智能助手\n\n" +
			personality,
		Instruction:     "需要知道当前时间时，调用 get_current_time 工具。",
		AdditionalTools: mcpTools,
	})

	events, err := s.Runner.Run(
		runContext,
		request.UserID,
		request.SessionID,
		message,
		opts...,
	)
	if err != nil {
		http.Error(w, "start agent run: "+err.Error(), http.StatusInternalServerError)
		return
	}
	run := usage.Run{
		RequestID: requestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	err = s.Usage.Start(r.Context(), run)
	if err != nil {
		http.Error(w, "start usage record: "+err.Error(), http.StatusInternalServerError)
		return
	}
	runFinished := false
	defer func() {
		if !runFinished {
			if err := s.Usage.Fail(context.Background(), requestID); err != nil {
				log.Printf("mark agent run %q failed: %v", requestID, err)
			}
		}
	}()

	writeSSEHeaders(w)
	builder := timeline.NewBuilder(requestID)
	if err := writeSSE(w, builder.Started()); err != nil {
		return
	}

	var tokens usage.Tokens
	for rawEvent := range events {
		if rawEvent.Response != nil && !rawEvent.Response.IsPartial &&
			!rawEvent.IsRunnerCompletion() {
			tokens.Add(rawEvent.Response.Usage, !definition.DoesNotReportCachedPromptTokens)
		}

		for _, streamEvent := range builder.Add(rawEvent) {
			if err := writeSSE(w, streamEvent); err != nil {
				return
			}
		}
		if rawEvent.IsRunnerCompletion() {
			if err := s.Usage.Complete(r.Context(), requestID, tokens); err != nil {
				log.Printf("complete agent run %q usage: %v", requestID, err)
				_ = writeSSE(w, timeline.DoneEvent{
					Type:      timeline.StreamEventTypeDone,
					RequestID: requestID,
				})
				return
			}
			runFinished = true

			var summary *usage.Summary
			if result, err := s.Usage.SessionSummary(r.Context(), request.UserID, request.SessionID); err != nil {
				log.Printf("summarize agent run %q usage: %v", requestID, err)
			} else {
				summary = &result
			}
			if err := writeSSE(w, timeline.DoneEvent{
				Type:         timeline.StreamEventTypeDone,
				RequestID:    requestID,
				SessionUsage: summary,
			}); err != nil {
				log.Printf("write agent run %q done event: %v", requestID, err)
			}
			return
		}
	}
}

func decodeAgentRunRequest(w http.ResponseWriter, r *http.Request) (AgentRunRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var request AgentRunRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid agent run request", http.StatusBadRequest)
		return AgentRunRequest{}, err
	}

	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	if request.UserID == "" || request.SessionID == "" || request.ModelID == "" ||
		(request.Message == "" && len(request.ImageIDs) == 0) {
		http.Error(w, "userId, sessionId, modelId, and either message or imageIds are required", http.StatusBadRequest)
		return AgentRunRequest{}, errors.New("missing required agent run fields")
	}
	return request, nil
}

func writeSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSE(w http.ResponseWriter, event timeline.StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode SSE event: %w", err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return fmt.Errorf("write SSE event: %w", err)
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
