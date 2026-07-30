package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// StreamMessage starts one Agent Run and returns EDITH's channel-neutral event
// stream. The caller must consume it until close; resource cleanup happens
// only after the framework event stream is fully drained.
func (s *Server) StreamMessage(
	taskContext context.Context,
	request MessageRequest,
) (<-chan StreamEvent, *APIError, int) {
	request, err := normalizeRequest(request)
	if err != nil {
		return nil, invalidRequest(err), 400
	}
	if taskContext == nil {
		taskContext = context.Background()
	}

	definition, ok := models.Lookup(request.ModelID)
	if !ok {
		return nil, invalidRequest(errors.New("unsupported modelId")), 400
	}
	if len(request.ImageIDs) > 0 && !definition.Info.Capabilities.Vision {
		return nil, invalidRequest(errors.New("selected model does not support image input")), 400
	}
	if !s.lanes.tryAcquire(request.UserID, request.SessionID, request.RequestID) {
		return nil, &APIError{Type: "session_busy", Message: "an agent run is already active for this session"}, 409
	}

	apiKey, err := s.users.LoadProviderAPIKey(taskContext, request.UserID, definition.ProviderID)
	if err != nil {
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, invalidRequest(fmt.Errorf("load model credential: %w", err)), 400
	}
	personality, err := s.users.LoadPersonality(taskContext, request.UserID)
	if err != nil {
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, internalError(fmt.Errorf("load user personality: %w", err)), 500
	}

	message := model.NewUserMessage(request.Message)
	taskContext = images.WithHydratedSession(taskContext)
	if len(request.ImageIDs) > 0 {
		taskContext, err = s.images.AddMessageImages(
			taskContext, request.UserID, request.SessionID, request.ImageIDs, &message,
		)
		if err != nil {
			s.lanes.release(request.UserID, request.SessionID, request.RequestID)
			return nil, invalidRequest(fmt.Errorf("prepare message images: %w", err)), 400
		}
	}

	mcpServers, err := s.users.LoadEnabledMCPServers(taskContext, request.UserID)
	if err != nil {
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, internalError(fmt.Errorf("load MCP servers: %w", err)), 500
	}
	mcpTools, closeMCP, err := mcp.OpenTools(taskContext, mcpServers)
	if err != nil {
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, internalError(fmt.Errorf("open MCP tools: %w", err)), 502
	}

	run := usage.Run{
		RequestID: request.RequestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	if err := usage.Start(s.usageDB, taskContext, run); err != nil {
		closeMCP()
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		if errors.Is(err, usage.ErrRunAlreadyExists) {
			return nil, &APIError{Type: "request_conflict", Message: "agent run already exists"}, 409
		}
		return nil, internalError(fmt.Errorf("start usage record: %w", err)), 500
	}

	opts := runopts.Build(runopts.Config{
		RequestID:         request.RequestID,
		Stream:            true,
		ModelName:         request.ModelID,
		APIKey:            apiKey,
		GlobalInstruction: "你是 EDITH AI Agent智能助手\n\n" + personality,
		Instruction:       "需要知道当前时间时，调用 get_current_time 工具。",
		AdditionalTools:   mcpTools,
	})
	events, err := s.runner.Run(taskContext, request.UserID, request.SessionID, message, opts...)
	if err != nil {
		_ = usage.Fail(s.usageDB, taskContext, request.RequestID)
		closeMCP()
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, internalError(fmt.Errorf("start agent run: %w", err)), 500
	}

	out := make(chan StreamEvent, 16)
	go s.forwardRun(taskContext, request, definition, run, events, out, closeMCP)
	return out, nil, 200
}

func (s *Server) forwardRun(
	taskContext context.Context,
	request MessageRequest,
	definition models.Definition,
	run usage.Run,
	events <-chan *event.Event,
	out chan<- StreamEvent,
	closeMCP func() error,
) {
	defer close(out)
	defer closeMCP()
	defer s.lanes.release(request.UserID, request.SessionID, request.RequestID)
	defer s.canceled.take(request.RequestID)

	finished := false
	defer func() {
		if finished {
			return
		}
		if err := usage.Fail(s.usageDB, taskContext, request.RequestID); err != nil {
			log.Printf("mark gateway agent run %q failed: %v", request.RequestID, err)
		}
	}()

	send := func(event StreamEvent) bool {
		select {
		case out <- event:
			return true
		case <-taskContext.Done():
			return false
		}
	}

	assistantID := "assistant_" + request.RequestID
	if !send(StreamEvent{Type: "run.started", SessionID: request.SessionID, RequestID: request.RequestID, AssistantID: assistantID}) {
		return
	}

	var (
		tokens        usage.Tokens
		nextBlockID   int
		lastBlockType string
		lastBlockID   string
		startedTools  = map[string]bool{}
	)
	for rawEvent := range events {
		if rawEvent == nil {
			continue
		}
		if rawEvent.IsError() {
			if !s.canceled.marked(request.RequestID) {
				send(StreamEvent{Type: "run.error", RequestID: request.RequestID, SessionID: request.SessionID, Error: &APIError{Type: "runner_error", Message: eventErrorMessage(rawEvent)}})
			}
			if !rawEvent.IsRunnerCompletion() {
				continue
			}
		}
		if rawEvent.IsRunnerCompletion() {
			summary, err := usage.Finish(s.usageDB, taskContext, run, tokens)
			if err != nil {
				log.Printf("finish gateway agent run %q usage: %v", request.RequestID, err)
				send(StreamEvent{Type: "run.error", RequestID: request.RequestID, SessionID: request.SessionID, Error: internalError(err)})
				return
			}
			finished = true
			eventType := "run.completed"
			if s.canceled.take(request.RequestID) {
				eventType = "run.canceled"
			}
			send(StreamEvent{Type: eventType, RequestID: request.RequestID, SessionID: request.SessionID, Usage: &summary})
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
				send(StreamEvent{Type: "reasoning.delta", AssistantID: assistantID, BlockID: lastBlockID, BlockType: "reasoning", Delta: choice.Delta.ReasoningContent})
			}
			if choice.Delta.Content != "" {
				if lastBlockType != "text" {
					nextBlockID++
					lastBlockType = "text"
					lastBlockID = assistantID + "_text_" + strconv.Itoa(nextBlockID)
				}
				send(StreamEvent{Type: "message.delta", AssistantID: assistantID, BlockID: lastBlockID, BlockType: "text", Delta: choice.Delta.Content})
			}
			if choice.Message.ToolID != "" {
				if !startedTools[choice.Message.ToolID] {
					send(StreamEvent{Type: "tool.started", AssistantID: assistantID, ToolCallID: choice.Message.ToolID, ToolName: choice.Message.ToolName, ToolStatus: "running"})
					startedTools[choice.Message.ToolID] = true
				}
				status := "completed"
				if rawEvent.Error != nil {
					status = "failed"
				}
				send(StreamEvent{Type: "tool.finished", AssistantID: assistantID, ToolCallID: choice.Message.ToolID, ToolStatus: status, ToolResult: choice.Message.Content})
				continue
			}
			if rawEvent.Response.IsPartial {
				continue
			}
			for _, call := range choice.Message.ToolCalls {
				if call.ID == "" || startedTools[call.ID] {
					continue
				}
				send(StreamEvent{Type: "tool.started", AssistantID: assistantID, ToolCallID: call.ID, ToolName: call.Function.Name, Arguments: string(call.Function.Arguments), ToolStatus: "running"})
				startedTools[call.ID] = true
			}
		}
		if !rawEvent.Response.IsPartial {
			usage.AddTokens(&tokens, rawEvent.Response.Usage, !definition.DoesNotReportCachedPromptTokens)
		}
	}
}

func normalizeRequest(request MessageRequest) (MessageRequest, error) {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	if request.RequestID == "" || request.UserID == "" || request.SessionID == "" || request.ModelID == "" {
		return MessageRequest{}, errors.New("requestId, userId, sessionId, and modelId are required")
	}
	if request.Message == "" && len(request.ImageIDs) == 0 {
		return MessageRequest{}, errors.New("message or imageIds is required")
	}
	return request, nil
}

func invalidRequest(err error) *APIError {
	return &APIError{Type: "invalid_request", Message: err.Error()}
}
func internalError(err error) *APIError {
	return &APIError{Type: "internal_error", Message: err.Error()}
}

func eventErrorMessage(source *event.Event) string {
	if source != nil && source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return "Agent 运行失败"
}
