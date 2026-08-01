package agentrun

import (
	"errors"
	"strings"

	"edith/backend-v2/internal/images"
	"edith/backend-v2/internal/models"
	"edith/backend-v2/internal/usage"
	"edith/backend-v2/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Dependencies 是 AgentRun 聚合一次执行所需的长期能力。
type Dependencies struct {
	Runner    runner.ManagedRunner
	Models    *models.Catalog
	Settings  *userconfig.Settings
	Providers *userconfig.Providers
	MCP       *userconfig.MCP
	Images    *images.AgentInput
	Usage     *usage.Recorder
}

// Service 是 EDITH 唯一的 Agent 执行入口。
type Service struct {
	runner         runner.ManagedRunner
	configurations *runConfigurations
	usage          *usage.Recorder
	lanes          *sessionLanes
	userStops      *userStops
}

// New 创建 AgentRun；内部小结构体在这里直接组装，不泄露给 main。
func New(deps Dependencies) (*Service, error) {
	if deps.Runner == nil || deps.Models == nil || deps.Settings == nil ||
		deps.Providers == nil || deps.MCP == nil || deps.Images == nil || deps.Usage == nil {
		return nil, errors.New("agentrun requires runner, models, settings, providers, MCP, images, and usage")
	}
	return &Service{
		runner: deps.Runner,
		configurations: &runConfigurations{
			models:    deps.Models,
			settings:  deps.Settings,
			providers: deps.Providers,
			mcp:       deps.MCP,
			images:    deps.Images,
		},
		usage:     deps.Usage,
		lanes:     &sessionLanes{active: make(map[sessionKey]string)},
		userStops: &userStops{requestIDs: make(map[string]struct{})},
	}, nil
}

// Run 按“会话准入 → 配置聚合 → ManagedRunner”启动一次任务。
func (s *Service) Run(request Request) (*Stream, *Error) {
	request = normalizeRequest(request)
	if validationError := validateRequest(request); validationError != nil {
		return nil, validationError
	}
	if !s.lanes.acquire(request.UserID, request.SessionID, request.RequestID) {
		return nil, &Error{Type: "session_busy", Message: "an agent run is already active for this session"}
	}

	configured, configError := s.configurations.Load(request)
	if configError != nil {
		s.lanes.release(request.UserID, request.SessionID, request.RequestID)
		return nil, configError
	}
	return s.startManagedRunner(configured)
}

// Status 查询 ManagedRunner 中属于该用户的活跃任务。
func (s *Service) Status(userID, requestID string) (Status, *Error) {
	userID, requestID = strings.TrimSpace(userID), strings.TrimSpace(requestID)
	if userID == "" || requestID == "" {
		return Status{}, &Error{Type: "invalid_request", Message: "userId and requestId are required"}
	}
	status, found := s.runner.RunStatus(requestID)
	if !found || status.SessionKey.UserID != userID {
		return Status{}, &Error{Type: "not_found", Message: "agent run not found"}
	}
	return Status{RequestID: requestID, Status: "running"}, nil
}

// Cancel 校验任务归属后向 ManagedRunner 发送用户停止信号。
func (s *Service) Cancel(userID, requestID string) *Error {
	userID, requestID = strings.TrimSpace(userID), strings.TrimSpace(requestID)
	if userID == "" || requestID == "" {
		return &Error{Type: "invalid_request", Message: "userId and requestId are required"}
	}
	status, found := s.runner.RunStatus(requestID)
	if !found || status.SessionKey.UserID != userID {
		return &Error{Type: "not_found", Message: "agent run not found"}
	}
	s.userStops.mark(requestID)
	if !s.runner.Cancel(requestID) {
		s.userStops.take(requestID)
		return &Error{Type: "not_found", Message: "agent run not found"}
	}
	return nil
}

func normalizeRequest(request Request) Request {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.UserID = strings.TrimSpace(request.UserID)
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.ModelID = strings.TrimSpace(request.ModelID)
	request.ReasoningOptionID = strings.TrimSpace(request.ReasoningOptionID)
	for index := range request.ImageIDs {
		request.ImageIDs[index] = strings.TrimSpace(request.ImageIDs[index])
	}
	return request
}

func validateRequest(request Request) *Error {
	if request.RequestID == "" || request.UserID == "" || request.SessionID == "" {
		return &Error{Type: "invalid_request", Message: "requestId, userId, and sessionId are required"}
	}
	if request.Message == "" && len(request.ImageIDs) == 0 {
		return &Error{Type: "invalid_request", Message: "message or imageIds is required"}
	}
	return nil
}
