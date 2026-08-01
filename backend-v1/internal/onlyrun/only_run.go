package onlyrun

import (
	"context"
	"database/sql"
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
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// OnlyRun 是 EDITH 唯一的 Agent Run 执行入口。
// 它持有执行所需的长期能力；所有用户、会话、消息等短期数据都经 Run 显式传入。
type OnlyRun struct {
	runner          runner.ManagedRunner
	users           *userconfig.Store
	images          *images.Service
	usageDB         *sql.DB
	lanes           *sessionLanes
	userCancelMarks *userCancelMarks
}

func New(
	runner runner.ManagedRunner,
	users *userconfig.Store,
	images *images.Service,
	usageDB *sql.DB,
) (*OnlyRun, error) {
	if runner == nil {
		return nil, errors.New("only run runner is required")
	}
	if users == nil {
		return nil, errors.New("only run user config store is required")
	}
	if images == nil {
		return nil, errors.New("only run image service is required")
	}
	if usageDB == nil {
		return nil, errors.New("only run usage database is required")
	}
	return &OnlyRun{
		runner:          runner,
		users:           users,
		images:          images,
		usageDB:         usageDB,
		lanes:           newSessionLanes(),
		userCancelMarks: newUserCancelMarks(),
	}, nil
}

// Run 启动一次渠道无关的 Agent 任务。
// 输入：已经由渠道适配器完成鉴权的消息请求。
// 输出：按顺序产生 EDITH StreamEvent 的 RunStream；启动失败时返回 APIError。
func (o *OnlyRun) Run(request MessageRequest) (*RunStream, *APIError) {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	if request.RequestID == "" || request.UserID == "" || request.SessionID == "" || request.ModelID == "" {
		return nil, &APIError{Type: "invalid_request", Message: "requestId, userId, sessionId, and modelId are required"}
	}
	if request.Message == "" && len(request.ImageIDs) == 0 {
		return nil, &APIError{Type: "invalid_request", Message: "message or imageIds is required"}
	}
	definition, ok := models.Lookup(request.ModelID)
	if !ok {
		return nil, &APIError{Type: "invalid_request", Message: "unsupported modelId"}
	}
	if len(request.ImageIDs) > 0 && !definition.Info.Capabilities.Vision {
		return nil, &APIError{Type: "invalid_request", Message: "selected model does not support image input"}
	}
	if !o.lanes.tryAcquire(request.UserID, request.SessionID, request.RequestID) {
		return nil, &APIError{Type: "session_busy", Message: "an agent run is already active for this session"}
	}

	handedOff := false
	var closeMCP func() error
	usageStarted := false
	taskContext := context.Background()
	defer func() {
		if handedOff {
			return
		}
		if usageStarted {
			if err := usage.Fail(o.usageDB, taskContext, request.RequestID); err != nil {
				log.Printf("mark agent run %q failed: %v", request.RequestID, err)
			}
		}
		if closeMCP != nil {
			if err := closeMCP(); err != nil {
				log.Printf("close MCP tools for %q: %v", request.RequestID, err)
			}
		}
		o.lanes.release(request.UserID, request.SessionID, request.RequestID)
	}()

	apiKey, err := o.users.LoadProviderAPIKey(taskContext, request.UserID, definition.ProviderID)
	if err != nil {
		return nil, &APIError{Type: "invalid_request", Message: fmt.Sprintf("load model credential: %v", err)}
	}
	personality, err := o.users.LoadPersonality(taskContext, request.UserID)
	if err != nil {
		return nil, &APIError{Type: "internal_error", Message: fmt.Sprintf("load user personality: %v", err)}
	}
	message := model.NewUserMessage(request.Message)
	taskContext = images.WithHydratedSession(taskContext)
	if len(request.ImageIDs) > 0 {
		taskContext, err = o.images.AddMessageImages(taskContext, request.UserID, request.SessionID, request.ImageIDs, &message)
		if err != nil {
			return nil, &APIError{Type: "invalid_request", Message: fmt.Sprintf("prepare message images: %v", err)}
		}
	}
	mcpServers, err := o.users.LoadEnabledMCPServers(taskContext, request.UserID)
	if err != nil {
		return nil, &APIError{Type: "internal_error", Message: fmt.Sprintf("load MCP servers: %v", err)}
	}
	mcpTools, closeMCP, err := mcp.OpenTools(taskContext, mcpServers)
	if err != nil {
		return nil, &APIError{Type: "internal_error", Message: fmt.Sprintf("open MCP tools: %v", err)}
	}
	run := usage.Run{
		RequestID: request.RequestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	if err := usage.Start(o.usageDB, taskContext, run); err != nil {
		if errors.Is(err, usage.ErrRunAlreadyExists) {
			return nil, &APIError{Type: "request_conflict", Message: "agent run already exists"}
		}
		return nil, &APIError{Type: "internal_error", Message: fmt.Sprintf("start usage record: %v", err)}
	}
	usageStarted = true
	opts := runopts.Build(runopts.Config{
		RequestID:         request.RequestID,
		Stream:            true,
		ModelName:         request.ModelID,
		APIKey:            apiKey,
		GlobalInstruction: "你是 EDITH AI Agent智能助手\n\n" + personality,
		Instruction:       "需要知道当前时间时，调用 get_current_time 工具。",
		AdditionalTools:   mcpTools,
	})
	rawEvents, err := o.runner.Run(taskContext, request.UserID, request.SessionID, message, opts...)
	if err != nil {
		return nil, &APIError{Type: "internal_error", Message: fmt.Sprintf("start agent run: %v", err)}
	}
	events := make(chan StreamEvent)
	handedOff = true
	go o.drainRunEvents(taskContext, request, definition.DoesNotReportCachedPromptTokens, run, rawEvents, events, closeMCP)
	return &RunStream{Events: events}, nil
}

// RunStatus 查询一个活跃任务，并校验该任务属于指定用户。
func (o *OnlyRun) RunStatus(userID, requestID string) (RunStatusResponse, *APIError) {
	userID = strings.TrimSpace(userID)
	requestID = strings.TrimSpace(requestID)
	if userID == "" || requestID == "" {
		return RunStatusResponse{}, &APIError{Type: "invalid_request", Message: "userId and requestId are required"}
	}
	status, ok := o.runner.RunStatus(requestID)
	if !ok || status.SessionKey.UserID != userID {
		return RunStatusResponse{}, &APIError{Type: "not_found", Message: "agent run not found"}
	}
	return RunStatusResponse{RequestID: requestID, Status: "running"}, nil
}

// Cancel 向 ManagedRunner 发送停止信号，并记录这是一次用户主动停止。
func (o *OnlyRun) Cancel(userID, requestID string) *APIError {
	userID = strings.TrimSpace(userID)
	requestID = strings.TrimSpace(requestID)
	if userID == "" || requestID == "" {
		return &APIError{Type: "invalid_request", Message: "userId and requestId are required"}
	}
	status, ok := o.runner.RunStatus(requestID)
	if !ok || status.SessionKey.UserID != userID || !o.runner.Cancel(requestID) {
		return &APIError{Type: "not_found", Message: "agent run not found"}
	}
	o.userCancelMarks.mark(requestID)
	return nil
}

// drainRunEvents 独占消费框架 eventCh，并转换为 EDITH 的中性 StreamEvent。
func (o *OnlyRun) drainRunEvents(
	taskContext context.Context,
	request MessageRequest,
	noCachedPromptTokens bool,
	run usage.Run,
	rawEvents <-chan *event.Event,
	events chan<- StreamEvent,
	closeMCP func() error,
) {
	defer close(events)
	defer o.lanes.release(request.UserID, request.SessionID, request.RequestID)
	defer o.userCancelMarks.take(request.RequestID)
	defer func() {
		if err := closeMCP(); err != nil {
			log.Printf("close MCP tools for %q: %v", request.RequestID, err)
		}
	}()
	finished := false
	defer func() {
		if finished {
			return
		}
		if err := usage.Fail(o.usageDB, taskContext, request.RequestID); err != nil {
			log.Printf("mark agent run %q failed: %v", request.RequestID, err)
		}
	}()
	assistantID := "assistant_" + request.RequestID
	events <- StreamEvent{Type: "run.started", SessionID: request.SessionID, RequestID: request.RequestID, AssistantID: assistantID}
	var tokens usage.Tokens
	nextBlockID := 0
	lastBlockType, lastBlockID := "", ""
	startedTools := map[string]bool{}
	for rawEvent := range rawEvents {
		if rawEvent == nil {
			continue
		}
		if rawEvent.IsError() && !o.userCancelMarks.marked(request.RequestID) {
			events <- StreamEvent{Type: "run.error", RequestID: request.RequestID, SessionID: request.SessionID, Error: &APIError{Type: "runner_error", Message: eventErrorMessage(rawEvent)}}
		}
		if rawEvent.IsRunnerCompletion() {
			summary, err := usage.Finish(o.usageDB, taskContext, run, tokens)
			if err != nil {
				events <- StreamEvent{Type: "run.error", RequestID: request.RequestID, SessionID: request.SessionID, Error: &APIError{Type: "internal_error", Message: err.Error()}}
				return
			}
			finished = true
			eventType := "run.completed"
			if o.userCancelMarks.take(request.RequestID) {
				eventType = "run.canceled"
			}
			events <- StreamEvent{Type: eventType, RequestID: request.RequestID, SessionID: request.SessionID, Usage: &summary}
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
				events <- StreamEvent{Type: "reasoning.delta", AssistantID: assistantID, BlockID: lastBlockID, BlockType: "reasoning", Delta: choice.Delta.ReasoningContent}
			}
			if choice.Delta.Content != "" {
				if lastBlockType != "text" {
					nextBlockID++
					lastBlockType = "text"
					lastBlockID = assistantID + "_text_" + strconv.Itoa(nextBlockID)
				}
				events <- StreamEvent{Type: "message.delta", AssistantID: assistantID, BlockID: lastBlockID, BlockType: "text", Delta: choice.Delta.Content}
			}
			if choice.Message.ToolID != "" {
				if !startedTools[choice.Message.ToolID] {
					events <- StreamEvent{Type: "tool.started", AssistantID: assistantID, ToolCallID: choice.Message.ToolID, ToolName: choice.Message.ToolName, ToolStatus: "running"}
					startedTools[choice.Message.ToolID] = true
				}
				status := "completed"
				if rawEvent.Error != nil {
					status = "failed"
				}
				events <- StreamEvent{Type: "tool.finished", AssistantID: assistantID, ToolCallID: choice.Message.ToolID, ToolStatus: status, ToolResult: choice.Message.Content}
				// Tool 是文本块之间的边界；Tool 后的文字必须生成新的 BlockID。
				lastBlockType, lastBlockID = "", ""
				continue
			}
			if rawEvent.Response.IsPartial {
				continue
			}
			toolStarted := false
			for _, call := range choice.Message.ToolCalls {
				if call.ID == "" || startedTools[call.ID] {
					continue
				}
				events <- StreamEvent{Type: "tool.started", AssistantID: assistantID, ToolCallID: call.ID, ToolName: call.Function.Name, Arguments: string(call.Function.Arguments), ToolStatus: "running"}
				startedTools[call.ID] = true
				toolStarted = true
			}
			if toolStarted {
				// Tool 是文本块之间的边界；Tool 后的文字必须生成新的 BlockID。
				lastBlockType, lastBlockID = "", ""
			}
		}
		if !rawEvent.Response.IsPartial {
			usage.AddTokens(&tokens, rawEvent.Response.Usage, !noCachedPromptTokens)
		}
	}
}

func eventErrorMessage(source *event.Event) string {
	if source != nil && source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return "Agent 运行失败"
}
