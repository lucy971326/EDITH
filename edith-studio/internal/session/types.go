package session

import "errors"

var (
	// ErrInvalidSessionID 表示会话标识为空或包含路径分隔符。
	ErrInvalidSessionID = errors.New("invalid session ID")
	// ErrSessionNotFound 表示当前 Workspace 中不存在该会话。
	ErrSessionNotFound = errors.New("session not found")
)

// Summary 是会话侧栏展示的一条会话摘要。
type Summary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

// History 是一次完整历史读取返回的会话摘要和聊天消息。
type History struct {
	Session  Summary       `json:"session"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage 是聊天时间线中的一条用户或助手消息。
type ChatMessage struct {
	ID      string           `json:"id"`
	Role    string           `json:"role"`
	Content string           `json:"content,omitempty"`
	Blocks  []AssistantBlock `json:"blocks,omitempty"`
}

// AssistantBlock 是助手消息中的思考、文本、工具或错误内容块。
type AssistantBlock struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty"`
	Status    string `json:"status,omitempty"`
}
