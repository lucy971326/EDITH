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
	taskContext, cancelTask := context.WithCancel(context.Background())
	defer cancelTask()

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
	apiKey, err := s.Users.LoadProviderAPIKey(taskContext, request.UserID, definition.ProviderID)
	if err != nil {
		http.Error(w, "load model credential: "+err.Error(), http.StatusBadRequest)
		return
	}
	personality, err := s.Users.LoadPersonality(taskContext, request.UserID)
	if err != nil {
		http.Error(w, "load user personality: "+err.Error(), http.StatusBadRequest)
		return
	}
	message := model.NewUserMessage(request.Message)
	taskContext = images.WithHydratedSession(taskContext)
	if len(request.ImageIDs) > 0 {
		if s.Images == nil {
			http.Error(w, "image service is unavailable", http.StatusServiceUnavailable)
			return
		}
		taskContext, err = s.Images.AddMessageImages(
			taskContext,
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

	mcpServers, err := s.Users.LoadEnabledMCPServers(taskContext, request.UserID)
	if err != nil {
		http.Error(w, "load MCP servers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mcpTools, closeMCP, err := mcp.OpenTools(taskContext, mcpServers)
	if err != nil {
		http.Error(w, "open MCP tools: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer closeMCP()

	requestID := request.RequestID
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

	run := usage.Run{
		RequestID: requestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	err = usage.Start(s.UsageDB, taskContext, run)
	if err != nil {
		if errors.Is(err, usage.ErrRunAlreadyExists) {
			http.Error(w, "agent run already exists", http.StatusConflict)
			return
		}
		http.Error(w, "start usage record: "+err.Error(), http.StatusInternalServerError)
		return
	}
	runFinished := false
	defer func() {
		if runFinished {
			return
		}

		failErr := usage.Fail(s.UsageDB, taskContext, requestID)
		if failErr == nil {
			return
		}

		log.Printf("mark agent run %q failed: %v", requestID, failErr)
	}()

	eventsCh, err := s.Runner.Run(
		taskContext,
		request.UserID,
		request.SessionID,
		message,
		opts...,
	)
	if err != nil {
		http.Error(w, "start agent run: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeSSEHeaders(w)
	clientConnected := true
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
	if clientConnected {
		writeErr := writeSSE(w, startedEvent)
		if writeErr != nil {
			clientConnected = false
		}
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
			if clientConnected {
				writeErr := writeSSE(w, errorEvent)
				if writeErr != nil {
					clientConnected = false
				}
			}
			if !rawEvent.IsRunnerCompletion() {
				continue
			}
		}

		if rawEvent.IsRunnerCompletion() {
			summary, finishErr := usage.Finish(s.UsageDB, taskContext, run, tokens)
			if finishErr != nil {
				log.Printf("finish agent run %q usage: %v", requestID, finishErr)
				doneEvent := DoneEvent{
					Type:      StreamEventTypeDone,
					RequestID: requestID,
				}
				if clientConnected {
					_ = writeSSE(w, doneEvent)
				}
				return
			}
			runFinished = true

			doneEvent := DoneEvent{
				Type:         StreamEventTypeDone,
				RequestID:    requestID,
				SessionUsage: &summary,
			}
			if clientConnected {
				writeErr := writeSSE(w, doneEvent)
				if writeErr != nil {
					log.Printf("write agent run %q done event: %v", requestID, writeErr)
				}
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
				if clientConnected {
					writeErr := writeSSE(w, deltaEvent)
					if writeErr != nil {
						clientConnected = false
					}
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
				if clientConnected {
					writeErr := writeSSE(w, deltaEvent)
					if writeErr != nil {
						clientConnected = false
					}
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
					if clientConnected {
						writeErr := writeSSE(w, startedEvent)
						if writeErr != nil {
							clientConnected = false
						}
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
				if clientConnected {
					writeErr := writeSSE(w, finishedEvent)
					if writeErr != nil {
						clientConnected = false
					}
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
				if clientConnected {
					writeErr := writeSSE(w, startedEvent)
					if writeErr != nil {
						clientConnected = false
					}
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
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	if _, err := uuid.Parse(request.RequestID); err != nil {
		http.Error(w, "requestId must be a UUID", http.StatusBadRequest)
		return AgentRunRequest{}, errors.New("invalid requestId")
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
