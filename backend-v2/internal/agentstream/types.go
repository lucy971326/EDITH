// Package agentstream 把框架事件解释为 EDITH 的渠道中性事件。
package agentstream

import "trpc.group/trpc-go/trpc-agent-go/model"

// APIError 描述一次任务启动或执行错误。
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// SessionUsage 是一次会话累计用量的渠道中性格式。
type SessionUsage struct {
	TotalTokens          int      `json:"totalTokens"`
	CachedPromptTokens   *int     `json:"cachedPromptTokens"`
	UncachedPromptTokens *int     `json:"uncachedPromptTokens"`
	CompletionTokens     int      `json:"completionTokens"`
	CacheHitRate         *float64 `json:"cacheHitRate"`
}

// Event 是 AgentRun 向所有渠道输出的唯一事件格式。
type Event struct {
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

	Usage *SessionUsage `json:"sessionUsage,omitempty"`
	Error *APIError     `json:"error,omitempty"`
}

// FrameworkEventResult 是单个框架事件的解释结果。
// AgentRun 使用完成、错误和用量信号管理任务生命周期。
type FrameworkEventResult struct {
	Events       []Event
	Usage        *model.Usage
	Completed    bool
	ErrorMessage string
}
