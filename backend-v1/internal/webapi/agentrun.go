package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/mcp"
	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/runopts"
	"edith/backend-v1/internal/usage"

	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func (s Server) runAgent(w http.ResponseWriter, r *http.Request) {
	request, err := decodeAgentRunRequest(w, r)
	if err != nil {
		return
	}
	if s.UsageDB == nil {
		http.Error(w, "usage service is unavailable", http.StatusServiceUnavailable)
		return
	}

	definition, ok := models.Lookup(request.ModelID)
	if !ok {
		http.Error(w, "unsupported modelId", http.StatusBadRequest)
		return
	}
	if len(request.ImageIDs) > 0 {
		if !definition.Info.Capabilities.Vision {
			http.Error(w, "selected model does not support image input", http.StatusBadRequest)
			return
		}
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
	defer closeMCP()

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

	eventsCh, err := s.Runner.Run(
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
	err = usage.Start(s.UsageDB, r.Context(), run)
	if err != nil {
		http.Error(w, "start usage record: "+err.Error(), http.StatusInternalServerError)
		return
	}
	runFinished := false
	defer func() {
		if runFinished {
			return
		}

		failErr := usage.Fail(s.UsageDB, context.Background(), requestID)
		if failErr == nil {
			return
		}

		log.Printf("mark agent run %q failed: %v", requestID, failErr)
	}()

	writeSSEHeaders(w)
	assistantID := "assistant_" + requestID
	startedEvent := AssistantStartedEvent{
		Type: StreamEventTypeAssistantStarted,
		Assistant: AssistantBlock{
			Type:      BlockTypeAssistant,
			ID:        assistantID,
			CreatedAt: time.Now(),
			Blocks:    []AssistantContentBlock{},
		},
	}
	err = writeSSE(w, startedEvent)
	if err != nil {
		return
	}

	var (
		tokens        usage.Tokens
		nextBlockID   int
		lastBlockType AssistantContentBlockType
		lastBlockID   string
		startedTools  = map[string]bool{}
	)

	for rawEvent := range eventsCh {
		if rawEvent == nil {
			continue
		}

		if rawEvent.IsError() {
			errorEvent := ErrorEvent{
				Type: StreamEventTypeError,
				Error: ErrorBlock{
					Type:      BlockTypeError,
					ID:        eventID(rawEvent, "error"),
					Message:   errorMessage(rawEvent),
					CreatedAt: eventTime(rawEvent),
				},
			}
			writeErr := writeSSE(w, errorEvent)
			if writeErr != nil {
				return
			}
			if !rawEvent.IsRunnerCompletion() {
				continue
			}
		}

		if rawEvent.IsRunnerCompletion() {
			summary, finishErr := usage.Finish(s.UsageDB, r.Context(), run, tokens)
			if finishErr != nil {
				log.Printf("finish agent run %q usage: %v", requestID, finishErr)
				doneEvent := DoneEvent{
					Type:      StreamEventTypeDone,
					RequestID: requestID,
				}
				_ = writeSSE(w, doneEvent)
				return
			}
			runFinished = true

			doneEvent := DoneEvent{
				Type:         StreamEventTypeDone,
				RequestID:    requestID,
				SessionUsage: &summary,
			}
			writeErr := writeSSE(w, doneEvent)
			if writeErr != nil {
				log.Printf("write agent run %q done event: %v", requestID, writeErr)
			}
			return
		}

		if rawEvent.Response == nil {
			continue
		}

		for _, choice := range rawEvent.Response.Choices {
			if choice.Delta.ReasoningContent != "" {
				if lastBlockType != AssistantContentBlockTypeReasoning {
					nextBlockID++
					lastBlockType = AssistantContentBlockTypeReasoning
					lastBlockID = assistantID + "_reasoning_" + strconv.Itoa(nextBlockID)
				}

				deltaEvent := ContentDeltaEvent{
					Type:        StreamEventTypeContentDelta,
					AssistantID: assistantID,
					BlockID:     lastBlockID,
					BlockType:   AssistantContentBlockTypeReasoning,
					Delta:       choice.Delta.ReasoningContent,
				}
				writeErr := writeSSE(w, deltaEvent)
				if writeErr != nil {
					return
				}
			}

			if choice.Delta.Content != "" {
				if lastBlockType != AssistantContentBlockTypeText {
					nextBlockID++
					lastBlockType = AssistantContentBlockTypeText
					lastBlockID = assistantID + "_text_" + strconv.Itoa(nextBlockID)
				}

				deltaEvent := ContentDeltaEvent{
					Type:        StreamEventTypeContentDelta,
					AssistantID: assistantID,
					BlockID:     lastBlockID,
					BlockType:   AssistantContentBlockTypeText,
					Delta:       choice.Delta.Content,
				}
				writeErr := writeSSE(w, deltaEvent)
				if writeErr != nil {
					return
				}
			}

			if choice.Message.ToolID != "" {
				if !startedTools[choice.Message.ToolID] {
					tool := AssistantContentBlock{
						Type:     AssistantContentBlockTypeTool,
						ID:       choice.Message.ToolID,
						ToolName: choice.Message.ToolName,
						Status:   ToolStatusRunning,
					}
					startedEvent := ToolStartedEvent{
						Type:        StreamEventTypeToolStarted,
						AssistantID: assistantID,
						Tool:        tool,
					}
					writeErr := writeSSE(w, startedEvent)
					if writeErr != nil {
						return
					}
					startedTools[choice.Message.ToolID] = true
					lastBlockType = AssistantContentBlockTypeTool
					lastBlockID = choice.Message.ToolID
				}

				status := ToolStatusCompleted
				if rawEvent.Error != nil {
					status = ToolStatusFailed
				}
				finishedEvent := ToolFinishedEvent{
					Type:        StreamEventTypeToolFinished,
					AssistantID: assistantID,
					ToolCallID:  choice.Message.ToolID,
					Status:      status,
					Result:      choice.Message.Content,
				}
				writeErr := writeSSE(w, finishedEvent)
				if writeErr != nil {
					return
				}
				continue
			}

			if rawEvent.Response.IsPartial {
				continue
			}

			for _, call := range choice.Message.ToolCalls {
				if call.ID == "" || startedTools[call.ID] {
					continue
				}

				tool := AssistantContentBlock{
					Type:      AssistantContentBlockTypeTool,
					ID:        call.ID,
					ToolName:  call.Function.Name,
					Arguments: string(call.Function.Arguments),
					Status:    ToolStatusRunning,
				}
				startedEvent := ToolStartedEvent{
					Type:        StreamEventTypeToolStarted,
					AssistantID: assistantID,
					Tool:        tool,
				}
				writeErr := writeSSE(w, startedEvent)
				if writeErr != nil {
					return
				}
				startedTools[call.ID] = true
				lastBlockType = AssistantContentBlockTypeTool
				lastBlockID = call.ID
			}
		}

		if rawEvent.Response.IsPartial {
			continue
		}

		usage.AddTokens(&tokens, rawEvent.Response.Usage, !definition.DoesNotReportCachedPromptTokens)
	}
}

func decodeAgentRunRequest(w http.ResponseWriter, r *http.Request) (AgentRunRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var request AgentRunRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&request)
	if err != nil {
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
	if request.UserID == "" {
		http.Error(w, "userId, sessionId, modelId, and either message or imageIds are required", http.StatusBadRequest)
		return AgentRunRequest{}, errors.New("missing required agent run fields")
	}
	if request.SessionID == "" {
		http.Error(w, "userId, sessionId, modelId, and either message or imageIds are required", http.StatusBadRequest)
		return AgentRunRequest{}, errors.New("missing required agent run fields")
	}
	if request.ModelID == "" {
		http.Error(w, "userId, sessionId, modelId, and either message or imageIds are required", http.StatusBadRequest)
		return AgentRunRequest{}, errors.New("missing required agent run fields")
	}
	if request.Message != "" {
		return request, nil
	}
	if len(request.ImageIDs) > 0 {
		return request, nil
	}

	http.Error(w, "userId, sessionId, modelId, and either message or imageIds are required", http.StatusBadRequest)
	return AgentRunRequest{}, errors.New("missing required agent run fields")
}

func writeSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSE(w http.ResponseWriter, event StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode SSE event: %w", err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return fmt.Errorf("write SSE event: %w", err)
	}

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
	return nil
}
