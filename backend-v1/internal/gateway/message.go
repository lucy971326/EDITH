package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/mcp"
	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/runopts"
	"edith/backend-v1/internal/usage"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// handleStreamMessage 处理一次完整的 Agent 消息执行。
// 输入：已鉴权的 MessageRequest JSON。
// 输出：执行进展以 SSE StreamEvent 持续返回；请求无法启动时返回 JSON 错误。
// 本函数直接启动 ManagedRunner 并读完框架 eventCh，不将事件流交给其他函数或 goroutine。
func (s *Server) handleStreamMessage(w http.ResponseWriter, r *http.Request) {
	var request MessageRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", "invalid gateway message request")
		return
	}

	request.RequestID = strings.TrimSpace(request.RequestID)
	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	if request.RequestID == "" || request.UserID == "" || request.SessionID == "" || request.ModelID == "" {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", "requestId, userId, sessionId, and modelId are required")
		return
	}
	if request.Message == "" && len(request.ImageIDs) == 0 {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", "message or imageIds is required")
		return
	}

	definition, ok := models.Lookup(request.ModelID)
	if !ok {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", "unsupported modelId")
		return
	}
	if len(request.ImageIDs) > 0 && !definition.Info.Capabilities.Vision {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", "selected model does not support image input")
		return
	}
	if !s.lanes.tryAcquire(request.UserID, request.SessionID, request.RequestID) {
		writeMessageError(w, http.StatusConflict, "session_busy", "an agent run is already active for this session")
		return
	}
	defer s.lanes.release(request.UserID, request.SessionID, request.RequestID)

	// 任务上下文刻意脱离 HTTP 请求上下文：浏览器断线只停止 SSE 写入，
	// 不能停止 Agent Run；主动停止只能通过 ManagedRunner.Cancel 完成。
	taskContext := context.Background()
	apiKey, err := s.users.LoadProviderAPIKey(taskContext, request.UserID, definition.ProviderID)
	if err != nil {
		writeMessageError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("load model credential: %v", err))
		return
	}
	personality, err := s.users.LoadPersonality(taskContext, request.UserID)
	if err != nil {
		writeMessageError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("load user personality: %v", err))
		return
	}

	message := model.NewUserMessage(request.Message)
	taskContext = images.WithHydratedSession(taskContext)
	if len(request.ImageIDs) > 0 {
		taskContext, err = s.images.AddMessageImages(
			taskContext, request.UserID, request.SessionID, request.ImageIDs, &message,
		)
		if err != nil {
			writeMessageError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("prepare message images: %v", err))
			return
		}
	}

	mcpServers, err := s.users.LoadEnabledMCPServers(taskContext, request.UserID)
	if err != nil {
		writeMessageError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("load MCP servers: %v", err))
		return
	}
	mcpTools, closeMCP, err := mcp.OpenTools(taskContext, mcpServers)
	if err != nil {
		writeMessageError(w, http.StatusBadGateway, "internal_error", fmt.Sprintf("open MCP tools: %v", err))
		return
	}
	defer func() {
		if err := closeMCP(); err != nil {
			log.Printf("close gateway MCP tools for %q: %v", request.RequestID, err)
		}
	}()

	run := usage.Run{
		RequestID: request.RequestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	if err := usage.Start(s.usageDB, taskContext, run); err != nil {
		if errors.Is(err, usage.ErrRunAlreadyExists) {
			writeMessageError(w, http.StatusConflict, "request_conflict", "agent run already exists")
			return
		}
		writeMessageError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("start usage record: %v", err))
		return
	}
	finished := false
	defer func() {
		if finished {
			return
		}
		if err := usage.Fail(s.usageDB, taskContext, request.RequestID); err != nil {
			log.Printf("mark gateway agent run %q failed: %v", request.RequestID, err)
		}
	}()

	opts := runopts.Build(runopts.Config{
		RequestID:         request.RequestID,
		Stream:            true,
		ModelName:         request.ModelID,
		APIKey:            apiKey,
		GlobalInstruction: "你是 EDITH AI Agent智能助手\n\n" + personality,
		Instruction:       "需要知道当前时间时，调用 get_current_time 工具。",
		AdditionalTools:   mcpTools,
	})
	eventCh, err := s.runner.Run(taskContext, request.UserID, request.SessionID, message, opts...)
	if err != nil {
		writeMessageError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("start agent run: %v", err))
		return
	}
	defer s.userCancelMarks.take(request.RequestID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	clientConnected := writeSSE(w, StreamEvent{
		Type:        "run.started",
		SessionID:   request.SessionID,
		RequestID:   request.RequestID,
		AssistantID: "assistant_" + request.RequestID,
	}) == nil

	assistantID := "assistant_" + request.RequestID
	var (
		tokens        usage.Tokens
		nextBlockID   int
		lastBlockType string
		lastBlockID   string
		startedTools  = map[string]bool{}
	)
	for rawEvent := range eventCh {
		if rawEvent == nil {
			continue
		}
		if rawEvent.IsError() && !s.userCancelMarks.marked(request.RequestID) && clientConnected {
			clientConnected = writeSSE(w, StreamEvent{
				Type:      "run.error",
				RequestID: request.RequestID,
				SessionID: request.SessionID,
				Error:     &APIError{Type: "runner_error", Message: eventErrorMessage(rawEvent)},
			}) == nil
		}
		if rawEvent.IsRunnerCompletion() {
			summary, err := usage.Finish(s.usageDB, taskContext, run, tokens)
			if err != nil {
				if clientConnected {
					_ = writeSSE(w, StreamEvent{
						Type:      "run.error",
						RequestID: request.RequestID,
						SessionID: request.SessionID,
						Error:     &APIError{Type: "internal_error", Message: err.Error()},
					})
				}
				return
			}
			finished = true
			eventType := "run.completed"
			if s.userCancelMarks.take(request.RequestID) {
				eventType = "run.canceled"
			}
			if clientConnected {
				_ = writeSSE(w, StreamEvent{
					Type:      eventType,
					RequestID: request.RequestID,
					SessionID: request.SessionID,
					Usage:     &summary,
				})
			}
			return
		}
		if rawEvent.Response == nil {
			continue
		}

		for _, choice := range rawEvent.Response.Choices {
			if choice.Delta.ReasoningContent != "" {
				if lastBlockType != "reasoning" {
					nextBlockID++
					lastBlockType = "reasoning"
					lastBlockID = assistantID + "_reasoning_" + strconv.Itoa(nextBlockID)
				}
				if clientConnected {
					clientConnected = writeSSE(w, StreamEvent{
						Type:        "reasoning.delta",
						AssistantID: assistantID,
						BlockID:     lastBlockID,
						BlockType:   "reasoning",
						Delta:       choice.Delta.ReasoningContent,
					}) == nil
				}
			}
			if choice.Delta.Content != "" {
				if lastBlockType != "text" {
					nextBlockID++
					lastBlockType = "text"
					lastBlockID = assistantID + "_text_" + strconv.Itoa(nextBlockID)
				}
				if clientConnected {
					clientConnected = writeSSE(w, StreamEvent{
						Type:        "message.delta",
						AssistantID: assistantID,
						BlockID:     lastBlockID,
						BlockType:   "text",
						Delta:       choice.Delta.Content,
					}) == nil
				}
			}
			if choice.Message.ToolID != "" {
				if !startedTools[choice.Message.ToolID] {
					if clientConnected {
						clientConnected = writeSSE(w, StreamEvent{
							Type:        "tool.started",
							AssistantID: assistantID,
							ToolCallID:  choice.Message.ToolID,
							ToolName:    choice.Message.ToolName,
							ToolStatus:  "running",
						}) == nil
					}
					startedTools[choice.Message.ToolID] = true
				}
				status := "completed"
				if rawEvent.Error != nil {
					status = "failed"
				}
				if clientConnected {
					clientConnected = writeSSE(w, StreamEvent{
						Type:        "tool.finished",
						AssistantID: assistantID,
						ToolCallID:  choice.Message.ToolID,
						ToolStatus:  status,
						ToolResult:  choice.Message.Content,
					}) == nil
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
				if clientConnected {
					clientConnected = writeSSE(w, StreamEvent{
						Type:        "tool.started",
						AssistantID: assistantID,
						ToolCallID:  call.ID,
						ToolName:    call.Function.Name,
						Arguments:   string(call.Function.Arguments),
						ToolStatus:  "running",
					}) == nil
				}
				startedTools[call.ID] = true
			}
		}
		if !rawEvent.Response.IsPartial {
			usage.AddTokens(&tokens, rawEvent.Response.Usage, !definition.DoesNotReportCachedPromptTokens)
		}
	}
}

func writeMessageError(w http.ResponseWriter, status int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIError{Type: errorType, Message: message})
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

func eventErrorMessage(source *event.Event) string {
	if source != nil && source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return "Agent 运行失败"
}
