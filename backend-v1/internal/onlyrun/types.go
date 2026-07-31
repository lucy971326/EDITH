// Package onlyrun 实现 EDITH 唯一的 Agent Run 执行入口。
package onlyrun

import "edith/backend-v1/internal/usage"

// MessageRequest 是一次已完成鉴权的 Agent 运行输入。
// 渠道适配器负责在进入 OnlyRun 前确认 UserID 可信。
type MessageRequest struct {
	RequestID         string   `json:"requestId"`
	UserID            string   `json:"userId"`
	SessionID         string   `json:"sessionId"`
	Message           string   `json:"message"`
	ImageIDs          []string `json:"imageIds"`
	ModelID           string   `json:"modelId"`
	ReasoningOptionID string   `json:"reasoningOptionId,omitempty"`
}

// APIError 是任务无法启动、执行失败或控制失败时输出的错误数据。
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamEvent 是一次 Agent Run 的渠道无关进度输出。
// OnlyRun 只输出事实事件，不决定浏览器 Timeline、IM 卡片或 GitHub 评论的展示形式。
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

// RunStream 是 OnlyRun 向 Gateway 交付一次任务进展的唯一出口。
// 渠道适配器即使失去外部连接，也必须继续读完 Events，保证后台任务自然收尾。
type RunStream struct {
	Events <-chan StreamEvent
}

// RunStatusResponse 是活跃任务查询的输出。只有 ManagedRunner 仍管理该任务时才返回。
type RunStatusResponse struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
}
