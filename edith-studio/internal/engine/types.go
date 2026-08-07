// Package engine 管理 EDITH 的 Agent 内核及其流式事件协议。
package engine

import "errors"

var (
	// ErrInvalidInput 表示一次 Run 缺少必要输入。
	ErrInvalidInput = errors.New("invalid run input")
	// ErrSessionBusy 表示同一 Session 已有任务正在运行。
	ErrSessionBusy = errors.New("session already has an active run")
	// ErrInvalidCompactInput 表示手动压缩缺少 Session 身份。
	ErrInvalidCompactInput = errors.New("invalid compact input")
)

// RunInput 是每次 Agent Run 都会变化的输入。
type RunInput struct {
	RequestID    string `json:"requestId"`
	SessionID    string `json:"sessionId"`
	Message      string `json:"message"`
	ModelID      string `json:"modelId,omitempty"`
	ThinkingMode string `json:"thinkingMode,omitempty"`
}

// CompactInput 是一次手动会话压缩的输入。
// 模型选择由 Web 随当前 Composer 状态传入，不在 Session 中另存一份。
type CompactInput struct {
	SessionID    string
	ModelID      string
	ThinkingMode string
}

// StreamEvent 是 Web 前端读取的稳定 SSE 事件。
type StreamEvent struct {
	Type       string       `json:"type"`
	BlockID    string       `json:"blockId,omitempty"`
	BlockType  string       `json:"blockType,omitempty"`
	Delta      string       `json:"delta,omitempty"`
	ToolCallID string       `json:"toolCallId,omitempty"`
	ToolName   string       `json:"toolName,omitempty"`
	Arguments  string       `json:"arguments,omitempty"`
	ToolStatus string       `json:"toolStatus,omitempty"`
	ToolResult string       `json:"toolResult,omitempty"`
	Error      *StreamError `json:"error,omitempty"`
}

// StreamError 是直接展示给用户的运行错误。
type StreamError struct {
	Message string `json:"message"`
}
