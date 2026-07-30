// Package timeline 定义 EDITH 面向前端的对话协议。
//
// 它不会直接暴露 trpc-agent-go 的 event.Event。实时 Runner 事件和持久化的
// Session 事件都会被转换为本包中的类型。
package timeline

import (
	"time"

	"edith/backend-v1/internal/usage"
)

type BlockType string

const (
	BlockTypeUser      BlockType = "user"
	BlockTypeAssistant BlockType = "assistant"
	BlockTypeError     BlockType = "error"
)

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

// 对话时间线 ------------------------------------------------------------------

// Timeline 是按时间顺序展示的一段对话。
type Timeline struct {
	Blocks []TimelineBlock `json:"blocks"`
}

// TimelineBlock 是 UserBlock、AssistantBlock 和 ErrorBlock 的联合类型。
// EDITH 只会生成它，不会从浏览器输入中解析它。
type TimelineBlock interface {
	isTimelineBlock()
}

// UserBlock 是用户发送的一条消息。
type UserBlock struct {
	Type      BlockType   `json:"type"`
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Images    []UserImage `json:"images"`
	CreatedAt time.Time   `json:"createdAt"`
}

func (UserBlock) isTimelineBlock() {}

// UserImage 只保存 EDITH 持久的图片 ID。浏览器通过需要鉴权的 BFF 接口读取图片，
// 而不会保存有时效的 COS URL。
type UserImage struct {
	ID string `json:"id"`
}

// AssistantBlock 是 Agent 的一轮回复。它的子块按用户看到的顺序保存推理、文本和
// 工具活动。
type AssistantBlock struct {
	Type      BlockType               `json:"type"`
	ID        string                  `json:"id"`
	CreatedAt time.Time               `json:"createdAt"`
	Blocks    []AssistantContentBlock `json:"blocks"`
}

func (AssistantBlock) isTimelineBlock() {}

// ErrorBlock 是对话时间线中用户可见的一次失败。
type ErrorBlock struct {
	Type      BlockType `json:"type"`
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ErrorBlock) isTimelineBlock() {}

// 助手回复内部的内容块 --------------------------------------------------------

// AssistantContentBlock 是由 Type 区分形状的 JSON 数据。工具块的 ID 使用框架
// ToolCall.ID，因此稍后到达的工具结果可以更新同一张工具卡片。
type AssistantContentBlock struct {
	Type    AssistantContentBlockType `json:"type"`
	ID      string                    `json:"id"`
	Content string                    `json:"content,omitempty"`

	ToolName  string     `json:"toolName,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
	Status    ToolStatus `json:"status,omitempty"`
	Result    string     `json:"result,omitempty"`
}

// 浏览器请求 ------------------------------------------------------------------

// ChatRequest 是浏览器发给 Next BFF 的请求。这里故意没有 userID：BFF 会从
// Clerk 登录态中取得它。
type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// 实时 SSE 事件 ---------------------------------------------------------------

type StreamEventType string

const (
	StreamEventTypeAssistantStarted StreamEventType = "assistant.started"
	StreamEventTypeContentDelta     StreamEventType = "assistant.content.delta"
	StreamEventTypeToolStarted      StreamEventType = "tool.started"
	StreamEventTypeToolFinished     StreamEventType = "tool.finished"
	StreamEventTypeError            StreamEventType = "error"
	StreamEventTypeDone             StreamEventType = "done"
)

// StreamEvent 是通过 EDITH SSE 发送的 JSON 联合类型。它与框架事件刻意分离，
// 因为框架事件的结构只是后端实现细节。
type StreamEvent interface {
	isStreamEvent()
}

// AssistantStartedEvent 为本次运行创建一个空的、可见的 AssistantBlock。
type AssistantStartedEvent struct {
	Type      StreamEventType `json:"type"`
	Assistant AssistantBlock  `json:"assistant"`
}

func (AssistantStartedEvent) isStreamEvent() {}

// ContentDeltaEvent 把 Delta 追加到指定的 reasoning 或 text 子块。
type ContentDeltaEvent struct {
	Type        StreamEventType           `json:"type"`
	AssistantID string                    `json:"assistantId"`
	BlockID     string                    `json:"blockId"`
	BlockType   AssistantContentBlockType `json:"blockType"`
	Delta       string                    `json:"delta"`
}

func (ContentDeltaEvent) isStreamEvent() {}

// ToolStartedEvent 创建一张运行中的工具卡片。Tool.ID 就是 ToolCall.ID。
type ToolStartedEvent struct {
	Type        StreamEventType       `json:"type"`
	AssistantID string                `json:"assistantId"`
	Tool        AssistantContentBlock `json:"tool"`
}

func (ToolStartedEvent) isStreamEvent() {}

// ToolFinishedEvent 更新由 ToolCallID 创建的那张工具卡片。
type ToolFinishedEvent struct {
	Type        StreamEventType `json:"type"`
	AssistantID string          `json:"assistantId"`
	ToolCallID  string          `json:"toolCallId"`
	Status      ToolStatus      `json:"status"`
	Result      string          `json:"result,omitempty"`
}

func (ToolFinishedEvent) isStreamEvent() {}

// ErrorEvent 在实时对话中加入一条用户可见的失败信息。
type ErrorEvent struct {
	Type  StreamEventType `json:"type"`
	Error ErrorBlock      `json:"error"`
}

func (ErrorEvent) isStreamEvent() {}

// DoneEvent 在 HTTP 层记录完用量后，结束一次 Agent 运行。
type DoneEvent struct {
	Type         StreamEventType `json:"type"`
	RequestID    string          `json:"requestId"`
	SessionUsage *usage.Summary  `json:"sessionUsage,omitempty"`
}

func (DoneEvent) isStreamEvent() {}
