// Package timeline defines EDITH's frontend-facing conversation contract.
//
// It deliberately does not expose trpc-agent-go event.Event. Both live Runner
// events and persisted Session events will be projected into these types.
package timeline

import "time"

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

// Timeline is a conversation rendered in chronological order.
type Timeline struct {
	Blocks []TimelineBlock `json:"blocks"`
}

// TimelineBlock is a JSON union of UserBlock, AssistantBlock, and ErrorBlock.
// EDITH only produces it; it does not decode it from browser input.
type TimelineBlock interface {
	isTimelineBlock()
}

// UserBlock is one message sent by a human user.
type UserBlock struct {
	Type      BlockType   `json:"type"`
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Images    []UserImage `json:"images"`
	CreatedAt time.Time   `json:"createdAt"`
}

func (UserBlock) isTimelineBlock() {}

// UserImage carries only EDITH's durable image identity. The browser resolves
// it through its authenticated BFF endpoint rather than storing COS URLs.
type UserImage struct {
	ID string `json:"id"`
}

// AssistantBlock is one Agent turn. Its child blocks preserve the visible
// order of reasoning, text, and tool activity within that turn.
type AssistantBlock struct {
	Type      BlockType               `json:"type"`
	ID        string                  `json:"id"`
	CreatedAt time.Time               `json:"createdAt"`
	Blocks    []AssistantContentBlock `json:"blocks"`
}

func (AssistantBlock) isTimelineBlock() {}

// ErrorBlock is a user-visible failure in the conversation timeline.
type ErrorBlock struct {
	Type      BlockType `json:"type"`
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ErrorBlock) isTimelineBlock() {}

// AssistantContentBlock is a discriminated JSON shape. Tool blocks use ID as
// the framework ToolCall.ID, which lets a later tool result update the exact
// same card.
type AssistantContentBlock struct {
	Type    AssistantContentBlockType `json:"type"`
	ID      string                    `json:"id"`
	Content string                    `json:"content,omitempty"`

	ToolName  string     `json:"toolName,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
	Status    ToolStatus `json:"status,omitempty"`
	Result    string     `json:"result,omitempty"`
}

// ChatRequest is the browser → Next BFF request. userID is intentionally not
// present: the BFF obtains it from Clerk.
type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type StreamEventType string

const (
	StreamEventTypeAssistantStarted StreamEventType = "assistant.started"
	StreamEventTypeContentDelta     StreamEventType = "assistant.content.delta"
	StreamEventTypeToolStarted      StreamEventType = "tool.started"
	StreamEventTypeToolFinished     StreamEventType = "tool.finished"
	StreamEventTypeError            StreamEventType = "error"
	StreamEventTypeDone             StreamEventType = "done"
)

// StreamEvent is the JSON union sent over EDITH SSE. It is intentionally
// separate from framework events, whose structure is an implementation detail.
type StreamEvent interface {
	isStreamEvent()
}

type AssistantStartedEvent struct {
	Type      StreamEventType `json:"type"`
	Assistant AssistantBlock  `json:"assistant"`
}

func (AssistantStartedEvent) isStreamEvent() {}

// ContentDeltaEvent appends Delta to one reasoning or text child block.
type ContentDeltaEvent struct {
	Type        StreamEventType           `json:"type"`
	AssistantID string                    `json:"assistantId"`
	BlockID     string                    `json:"blockId"`
	BlockType   AssistantContentBlockType `json:"blockType"`
	Delta       string                    `json:"delta"`
}

func (ContentDeltaEvent) isStreamEvent() {}

type ToolStartedEvent struct {
	Type        StreamEventType       `json:"type"`
	AssistantID string                `json:"assistantId"`
	Tool        AssistantContentBlock `json:"tool"`
}

func (ToolStartedEvent) isStreamEvent() {}

type ToolFinishedEvent struct {
	Type        StreamEventType `json:"type"`
	AssistantID string          `json:"assistantId"`
	ToolCallID  string          `json:"toolCallId"`
	Status      ToolStatus      `json:"status"`
	Result      string          `json:"result,omitempty"`
}

func (ToolFinishedEvent) isStreamEvent() {}

type ErrorEvent struct {
	Type  StreamEventType `json:"type"`
	Error ErrorBlock      `json:"error"`
}

func (ErrorEvent) isStreamEvent() {}

type DoneEvent struct {
	Type      StreamEventType `json:"type"`
	RequestID string          `json:"requestId"`
	Usage     *Usage          `json:"usage,omitempty"`
}

func (DoneEvent) isStreamEvent() {}

// Usage is the user-visible token accounting captured during one Run.
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}
