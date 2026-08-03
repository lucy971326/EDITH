package gateway

import (
	"context"
	"errors"
	"strings"

	"edith/backend-v2/internal/agentrun"
	"edith/backend-v2/internal/userconfig"
)

// Dependencies 是 Gateway 的两个明确下游能力。
type Dependencies struct {
	Bindings  *userconfig.Bindings
	AgentRuns *agentrun.Service
}

// Service 是所有 Agent 渠道的唯一入口。
type Service struct {
	bindings  *userconfig.Bindings
	agentRuns *agentrun.Service
}

// New 创建 Gateway；它不会创建 AgentRun 或渠道 Adapter。
func New(deps Dependencies) (*Service, error) {
	if deps.Bindings == nil || deps.AgentRuns == nil {
		return nil, errors.New("gateway requires bindings and agent runs")
	}
	return &Service{bindings: deps.Bindings, agentRuns: deps.AgentRuns}, nil
}

// Run 把渠道身份与会话翻译为 EDITH 身份后启动 AgentRun。
func (g *Service) Run(input IncomingMessage) (*Stream, *Error) {
	input.Channel = strings.TrimSpace(input.Channel)
	input.ExternalUserID = strings.TrimSpace(input.ExternalUserID)
	input.SessionKey = strings.TrimSpace(input.SessionKey)
	if input.Channel == "" || input.ExternalUserID == "" {
		return nil, &Error{Type: "invalid_request", Message: "channel and externalUserId are required"}
	}

	userID := input.ExternalUserID
	sessionID := input.SessionKey
	if !isTrustedChannel(input.Channel) {
		boundUserID, found, err := g.bindings.ToClerkUserID(context.Background(), input.Channel, input.ExternalUserID)
		if err != nil {
			return nil, &Error{Type: "internal_error", Message: err.Error()}
		}
		if !found {
			return nil, &Error{Type: "identity_not_bound", Message: "channel user is not bound to an EDITH user"}
		}
		userID = boundUserID
		sessionID = input.Channel + ":" + userID
	}
	if sessionID == "" {
		return nil, &Error{Type: "invalid_request", Message: "sessionKey is required for trusted channels"}
	}

	return g.agentRuns.Run(agentrun.Request{
		RequestID: input.RequestID, UserID: userID, SessionID: sessionID,
		Message: input.Message, ImageIDs: input.ImageIDs, ModelID: input.ModelID,
		UploadPaths:       input.UploadPaths,
		ReasoningOptionID: input.ReasoningOptionID,
	})
}

// Status 查询一个属于可信 Clerk 用户的活跃任务。
func (g *Service) Status(userID, requestID string) (Status, *Error) {
	return g.agentRuns.Status(userID, requestID)
}

// Cancel 停止一个属于可信 Clerk 用户的活跃任务。
func (g *Service) Cancel(userID, requestID string) *Error {
	return g.agentRuns.Cancel(userID, requestID)
}

func isTrustedChannel(channel string) bool {
	return channel == WebChannel || channel == CronChannel
}
