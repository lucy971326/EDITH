// Package gateway 对外提供 EDITH 渠道无关的 Agent 消息协议。
package gateway

import "edith/backend-v1/internal/usage"

// MessageRequest 是一次已完成鉴权的 Agent 运行输入。
// 输入包含用户、会话、请求身份和消息内容；Web BFF 与未来渠道适配器负责在进入
// Gateway 前确认 UserID 可信，Gateway 不从浏览器直接接收未鉴权的 UserID。
type MessageRequest struct {
	RequestID         string   `json:"requestId"`
	UserID            string   `json:"userId"`
	SessionID         string   `json:"sessionId"`
	Message           string   `json:"message"`
	ImageIDs          []string `json:"imageIds"`
	ModelID           string   `json:"modelId"`
	ReasoningOptionID string   `json:"reasoningOptionId,omitempty"`
}

// APIError 是 Gateway 在请求未能启动或运行过程中输出的错误数据。
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamEvent 是一次 Agent Run 的渠道无关进度输出。
// Gateway 只输出事实事件，不决定浏览器 Timeline、IM 卡片或 GitHub 评论的展示形式；
// 每个渠道自行将这些事件投影为自己的 UI。
type StreamEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
	RequestID string `json:"requestId,omitempty"`

	AssistantID string `json:"assistantId,omitempty"`
	BlockID     string `json:"blockId,omitempty"`
	BlockType   string `json:"blockType,omitempty"`
	Delta       string `json:"delta,omitempty"`

	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	ToolStatus string `json:"toolStatus,omitempty"`
	ToolResult string `json:"toolResult,omitempty"`

	Usage *usage.Summary `json:"sessionUsage,omitempty"`
	Error *APIError      `json:"error,omitempty"`
}

// RunStatusResponse 是活跃任务查询的输出。只有 ManagedRunner 仍管理该任务时才返回。
type RunStatusResponse struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
}
