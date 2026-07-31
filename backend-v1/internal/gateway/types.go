// Package gateway 对外提供 EDITH 渠道无关的 Agent 消息协议。
package gateway

import "edith/backend-v1/internal/onlyrun"

const WebChannel = "web"

// IncomingMessage 是渠道适配器交给 Gateway 的渠道事实。
// ExternalUserID 和 SessionKey 尚未是 EDITH 身份；Gateway 负责将它们映射为
// Clerk 用户 ID 与 EDITH Session ID，再交给 OnlyRun 执行。
type IncomingMessage struct {
	Channel           string
	ExternalUserID    string
	SessionKey        string
	RequestID         string
	Message           string
	ImageIDs          []string
	ModelID           string
	ReasoningOptionID string
}

// 以下别名让渠道只依赖 Gateway 契约；实际执行契约由 OnlyRun 定义。
type MessageRequest = onlyrun.MessageRequest
type APIError = onlyrun.APIError
type StreamEvent = onlyrun.StreamEvent
type RunStream = onlyrun.RunStream
type RunStatusResponse = onlyrun.RunStatusResponse
