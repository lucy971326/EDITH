package webapi

import (
	"time"

	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/usage"
)

// CreateImageUploadRequest asks Go to reserve one image and issue its short
// lived COS upload URL. userId is injected by the BFF.
type CreateImageUploadRequest struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// ChatImage is the durable browser-facing image identity. COS object keys and
// presigned URLs stay inside Go.
type ChatImage struct {
	ID       string `json:"id"`
	MimeType string `json:"mimeType"`
}

type CreateImageUploadResponse struct {
	Image     ChatImage `json:"image"`
	UploadURL string    `json:"uploadUrl"`
}

type CompleteImageUploadRequest struct {
	UserID string `json:"userId"`
}

type CompleteImageUploadResponse struct {
	Image ChatImage `json:"image"`
}

// ModelCatalogResponse is the Go Runtime → Next BFF response body for the
// model catalog. The wrapper leaves room for future catalog metadata.
type ModelCatalogResponse struct {
	Providers []models.ProviderInfo `json:"providers"`
	Models    []models.Info         `json:"models"`
}

// UserSettingsRequest is the Next BFF → Go Runtime save request. An omitted
// apiKey preserves the existing key; userId is injected by the BFF.
type UserSettingsRequest struct {
	UserID         string                    `json:"userId"`
	Personality    string                    `json:"personality"`
	DefaultModelID string                    `json:"defaultModelId"`
	Providers      []ProviderCredentialInput `json:"providers"`
}

type ProviderCredentialInput struct {
	ProviderID string  `json:"providerId"`
	APIKey     *string `json:"apiKey"`
}

// UserSettingsResponse is the safe Go Runtime → Next BFF settings response.
// API keys never leave the Go Runtime after being stored.
type UserSettingsResponse struct {
	Personality    string                    `json:"personality"`
	DefaultModelID string                    `json:"defaultModelId"`
	Providers      []ProviderCredentialState `json:"providers"`
}

type ProviderCredentialState struct {
	ProviderID string `json:"providerId"`
	HasAPIKey  bool   `json:"hasApiKey"`
}

// MCPServerRequest is the BFF → Go Runtime request body for one MCP server.
// userId is injected by the BFF. Header values are write-only secrets.
type MCPServerRequest struct {
	UserID    string           `json:"userId"`
	Name      string           `json:"name"`
	URL       string           `json:"url"`
	Transport string           `json:"transport"`
	Enabled   bool             `json:"enabled"`
	Headers   []MCPHeaderInput `json:"headers"`
}

type MCPHeaderInput struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

// MCPServerResponse is safe for the browser. Header values never leave Go.
type MCPServerResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	URL       string           `json:"url"`
	Transport string           `json:"transport"`
	Enabled   bool             `json:"enabled"`
	Headers   []MCPHeaderState `json:"headers"`
}

type MCPHeaderState struct {
	Name     string `json:"name"`
	HasValue bool   `json:"hasValue"`
}

type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}

// Conversation is the lightweight summary used by the sidebar. Title is
// derived from the first user message, not stored as separate metadata.
type Conversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

type ConversationListResponse struct {
	Conversations []Conversation `json:"conversations"`
}

type ConversationResponse struct {
	Timeline Timeline      `json:"timeline"`
	Usage    usage.Summary `json:"usage"`
}

// Timeline 是浏览器按时间顺序展示的一段对话。
type Timeline struct {
	Blocks []TimelineBlock `json:"blocks"`
}

// TimelineBlock 是用户消息、AI 回复或错误卡片。
type TimelineBlock interface {
	isTimelineBlock()
}

type BlockType string

const (
	BlockTypeUser      BlockType = "user"
	BlockTypeAssistant BlockType = "assistant"
	BlockTypeError     BlockType = "error"
)

type UserBlock struct {
	Type      BlockType   `json:"type"`
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Images    []UserImage `json:"images"`
	CreatedAt time.Time   `json:"createdAt"`
}

func (UserBlock) isTimelineBlock() {}

type UserImage struct {
	ID string `json:"id"`
}

type AssistantBlock struct {
	Type      BlockType               `json:"type"`
	ID        string                  `json:"id"`
	CreatedAt time.Time               `json:"createdAt"`
	Blocks    []AssistantContentBlock `json:"blocks"`
}

func (AssistantBlock) isTimelineBlock() {}

type ErrorBlock struct {
	Type      BlockType `json:"type"`
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ErrorBlock) isTimelineBlock() {}

type AssistantContentBlockType string

const (
	AssistantContentBlockTypeReasoning AssistantContentBlockType = "reasoning"
	AssistantContentBlockTypeText      AssistantContentBlockType = "text"
	AssistantContentBlockTypeTool      AssistantContentBlockType = "tool"
)

type ToolStatus string

const (
	ToolStatusRunning   ToolStatus = "running"
	ToolStatusCompleted ToolStatus = "completed"
	ToolStatusFailed    ToolStatus = "failed"
)

type AssistantContentBlock struct {
	Type    AssistantContentBlockType `json:"type"`
	ID      string                    `json:"id"`
	Content string                    `json:"content,omitempty"`

	ToolName  string     `json:"toolName,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
	Status    ToolStatus `json:"status,omitempty"`
	Result    string     `json:"result,omitempty"`
}
