package gateway

import (
	"context"
	"strings"

	"edith/backend-v1/internal/models"
)

// Run 将渠道事实映射为 EDITH 用户、会话和模型，再交给唯一执行入口。
// 输入：渠道名、外部用户标识、会话定位信息与消息。
// 输出：OnlyRun 的中性事件流；未绑定的外部账号不会启动 Agent。
func (g *Gateway) Run(input IncomingMessage) (*RunStream, *APIError) {
	request, apiError := g.resolveMessage(input)
	if apiError != nil {
		return nil, apiError
	}
	return g.onlyRun.Run(request)
}

// resolveMessage 将渠道事实翻译为 OnlyRun 所需的 EDITH 消息。
// 输入：渠道名、外部用户标识、会话定位信息和消息。
// 输出：带 Clerk 用户 ID、EDITH Session ID 与最终模型 ID 的 MessageRequest。
func (g *Gateway) resolveMessage(input IncomingMessage) (MessageRequest, *APIError) {
	input.Channel = strings.TrimSpace(input.Channel)
	input.ExternalUserID = strings.TrimSpace(input.ExternalUserID)
	input.SessionKey = strings.TrimSpace(input.SessionKey)
	if input.Channel == "" || input.ExternalUserID == "" {
		return MessageRequest{}, &APIError{Type: "invalid_request", Message: "channel and externalUserId are required"}
	}

	userID := input.ExternalUserID
	sessionID := input.SessionKey
	if input.Channel != WebChannel {
		boundUserID, found, err := g.users.LookupChannelUser(context.Background(), input.Channel, input.ExternalUserID)
		if err != nil {
			return MessageRequest{}, &APIError{Type: "internal_error", Message: err.Error()}
		}
		if !found {
			return MessageRequest{}, &APIError{Type: "identity_not_bound", Message: "channel user is not bound to an EDITH user"}
		}
		userID = boundUserID
		sessionID = input.Channel + ":" + userID
	}
	if sessionID == "" {
		return MessageRequest{}, &APIError{Type: "invalid_request", Message: "sessionKey is required for web messages"}
	}

	modelID := strings.TrimSpace(input.ModelID)
	if modelID == "" {
		storedModelID, err := g.users.LoadDefaultModelID(context.Background(), userID)
		if err != nil {
			return MessageRequest{}, &APIError{Type: "internal_error", Message: err.Error()}
		}
		modelID = storedModelID
	}
	if modelID == "" {
		modelID = models.DefaultModelID
	}

	return MessageRequest{
		RequestID:         input.RequestID,
		UserID:            userID,
		SessionID:         sessionID,
		Message:           input.Message,
		ImageIDs:          input.ImageIDs,
		ModelID:           modelID,
		ReasoningOptionID: input.ReasoningOptionID,
	}, nil
}
