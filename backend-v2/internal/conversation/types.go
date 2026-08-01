package conversation

import (
	"time"

	"edith/backend-v2/internal/usage"
)

// Conversation 是侧边栏展示的一段会话摘要。
type Conversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

// ConversationListResponse 是会话列表 HTTP 响应。
type ConversationListResponse struct {
	Conversations []Conversation `json:"conversations"`
}

// ConversationResponse 是一个会话的 Timeline 和用量 HTTP 响应。
type ConversationResponse struct {
	Timeline Timeline      `json:"timeline"`
	Usage    usage.Summary `json:"usage"`
}

// Timeline 是浏览器按时间顺序展示的一段对话。
type Timeline struct {
	Blocks []TimelineBlock `json:"blocks"`
}

// TimelineBlock 是用户消息、AI 回复或错误卡片。
type TimelineBlock interface{ isTimelineBlock() }

// UserBlock 是一条用户消息卡片。
type UserBlock struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Images    []UserImage `json:"images"`
	CreatedAt time.Time   `json:"createdAt"`
}

func (UserBlock) isTimelineBlock() {}

// UserImage 是用户消息中引用的 EDITH 图片。
type UserImage struct {
	ID string `json:"id"`
}

// AssistantBlock 是一整段 AI 回复卡片。
type AssistantBlock struct {
	Type      string                  `json:"type"`
	ID        string                  `json:"id"`
	CreatedAt time.Time               `json:"createdAt"`
	Blocks    []AssistantContentBlock `json:"blocks"`
}

func (AssistantBlock) isTimelineBlock() {}

// ErrorBlock 是一次历史运行错误卡片。
type ErrorBlock struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ErrorBlock) isTimelineBlock() {}

// AssistantContentBlock 是 reasoning、text 或 tool 的展示块。
type AssistantContentBlock struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Content   string `json:"content,omitempty"`
	ToolName  string `json:"toolName,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Status    string `json:"status,omitempty"`
	Result    string `json:"result,omitempty"`
}
