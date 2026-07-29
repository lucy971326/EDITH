package timeline

import (
	"fmt"
	"strconv"
	"time"

	"edith/backend-v1/internal/images"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// Builder holds only the short-lived state required to turn one Runner.Run
// event stream into one AssistantBlock.
type Builder struct {
	assistant AssistantBlock
	toolIndex map[string]int
	nextID    int

	sawPartialReasoning bool
	sawPartialText      bool
}

// NewBuilder starts one visible Agent turn.
func NewBuilder(requestID string) *Builder {
	return newBuilder(requestID, time.Now())
}

func newBuilder(requestID string, createdAt time.Time) *Builder {
	assistantID := "assistant_" + requestID
	return &Builder{
		assistant: AssistantBlock{
			Type:      BlockTypeAssistant,
			ID:        assistantID,
			CreatedAt: createdAt,
			// The first SSE frame is rendered immediately. Keep its JSON field
			// stable as [] rather than null before any content has arrived.
			Blocks: []AssistantContentBlock{},
		},
		toolIndex: make(map[string]int),
	}
}

// Started returns the SSE event that creates this Builder's AssistantBlock on
// the browser. Call it once before forwarding events returned by Add.
func (b *Builder) Started() AssistantStartedEvent {
	return AssistantStartedEvent{
		Type:      StreamEventTypeAssistantStarted,
		Assistant: b.Assistant(),
	}
}

// Add translates one framework Event into zero or more EDITH SSE events.
// Runner completion is deliberately ignored here; the HTTP layer emits done
// after it has captured final usage.
func (b *Builder) Add(source *event.Event) []StreamEvent {
	if source == nil || source.IsRunnerCompletion() {
		return nil
	}

	if source.IsError() {
		return []StreamEvent{ErrorEvent{
			Type: StreamEventTypeError,
			Error: ErrorBlock{
				Type:      BlockTypeError,
				ID:        eventID(source, "error"),
				Message:   errorMessage(source),
				CreatedAt: eventTime(source),
			},
		}}
	}
	if source.Response == nil {
		return nil
	}

	var output []StreamEvent
	for _, choice := range source.Response.Choices {
		if choice.Delta.ReasoningContent != "" {
			output = append(output, b.appendDelta(
				AssistantContentBlockTypeReasoning,
				choice.Delta.ReasoningContent,
			))
			b.sawPartialReasoning = true
		}
		if choice.Delta.Content != "" {
			output = append(output, b.appendDelta(
				AssistantContentBlockTypeText,
				choice.Delta.Content,
			))
			b.sawPartialText = true
		}

		if choice.Message.ToolID != "" {
			output = append(output, b.finishTool(choice.Message, source.Error != nil)...)
			continue
		}

		if source.IsPartial {
			continue
		}

		if choice.Message.ReasoningContent != "" && !b.sawPartialReasoning {
			b.appendComplete(AssistantContentBlockTypeReasoning, choice.Message.ReasoningContent)
		}
		if choice.Message.Content != "" && !b.sawPartialText {
			b.appendComplete(AssistantContentBlockTypeText, choice.Message.Content)
		}
		for _, call := range choice.Message.ToolCalls {
			if started, ok := b.startTool(call); ok {
				output = append(output, started)
			}
		}
	}

	if !source.IsPartial {
		b.sawPartialReasoning = false
		b.sawPartialText = false
	}
	return output
}

// Assistant returns the current complete-in-progress visible Agent turn.
func (b *Builder) Assistant() AssistantBlock {
	return b.assistant
}

func (b *Builder) appendDelta(kind AssistantContentBlockType, delta string) ContentDeltaEvent {
	index := b.currentContentBlock(kind)
	b.assistant.Blocks[index].Content += delta

	return ContentDeltaEvent{
		Type:        StreamEventTypeContentDelta,
		AssistantID: b.assistant.ID,
		BlockID:     b.assistant.Blocks[index].ID,
		BlockType:   kind,
		Delta:       delta,
	}
}

func (b *Builder) appendComplete(kind AssistantContentBlockType, content string) {
	index := b.currentContentBlock(kind)
	b.assistant.Blocks[index].Content += content
}

func (b *Builder) currentContentBlock(kind AssistantContentBlockType) int {
	last := len(b.assistant.Blocks) - 1
	if last >= 0 && b.assistant.Blocks[last].Type == kind {
		return last
	}

	b.assistant.Blocks = append(b.assistant.Blocks, AssistantContentBlock{
		Type: kind,
		ID:   b.blockID(kind),
	})
	return len(b.assistant.Blocks) - 1
}

func (b *Builder) startTool(call model.ToolCall) (ToolStartedEvent, bool) {
	if call.ID == "" {
		return ToolStartedEvent{}, false
	}
	if _, exists := b.toolIndex[call.ID]; exists {
		return ToolStartedEvent{}, false
	}

	tool := AssistantContentBlock{
		Type:      AssistantContentBlockTypeTool,
		ID:        call.ID,
		ToolName:  call.Function.Name,
		Arguments: string(call.Function.Arguments),
		Status:    ToolStatusRunning,
	}
	b.assistant.Blocks = append(b.assistant.Blocks, tool)
	b.toolIndex[call.ID] = len(b.assistant.Blocks) - 1

	return ToolStartedEvent{
		Type:        StreamEventTypeToolStarted,
		AssistantID: b.assistant.ID,
		Tool:        tool,
	}, true
}

func (b *Builder) finishTool(message model.Message, failed bool) []StreamEvent {
	index, exists := b.toolIndex[message.ToolID]
	var output []StreamEvent
	if !exists {
		tool := AssistantContentBlock{
			Type:     AssistantContentBlockTypeTool,
			ID:       message.ToolID,
			ToolName: message.ToolName,
			Status:   ToolStatusRunning,
		}
		b.assistant.Blocks = append(b.assistant.Blocks, tool)
		index = len(b.assistant.Blocks) - 1
		b.toolIndex[message.ToolID] = index
		output = append(output, ToolStartedEvent{
			Type:        StreamEventTypeToolStarted,
			AssistantID: b.assistant.ID,
			Tool:        tool,
		})
	}

	status := ToolStatusCompleted
	if failed {
		status = ToolStatusFailed
	}
	b.assistant.Blocks[index].Status = status
	b.assistant.Blocks[index].Result = message.Content

	output = append(output, ToolFinishedEvent{
		Type:        StreamEventTypeToolFinished,
		AssistantID: b.assistant.ID,
		ToolCallID:  message.ToolID,
		Status:      status,
		Result:      message.Content,
	})
	return output
}

func (b *Builder) blockID(kind AssistantContentBlockType) string {
	b.nextID++
	return b.assistant.ID + "_" + string(kind) + "_" + strconv.Itoa(b.nextID)
}

// BuildHistory projects complete Session events into the same Timeline that
// the browser reaches after consuming live SSE events.
func BuildHistory(events []event.Event) Timeline {
	var (
		timeline Timeline
		active   *Builder
	)

	flushActive := func() {
		if active == nil || len(active.assistant.Blocks) == 0 {
			return
		}
		timeline.Blocks = append(timeline.Blocks, active.Assistant())
		active = nil
	}

	for index := range events {
		source := &events[index]
		if source.Response == nil || source.IsRunnerCompletion() {
			continue
		}

		if source.Response.IsUserMessage() {
			flushActive()
			for _, choice := range source.Response.Choices {
				if choice.Message.Role != model.RoleUser {
					continue
				}
				timeline.Blocks = append(timeline.Blocks, UserBlock{
					Type:      BlockTypeUser,
					ID:        eventID(source, "user"),
					Content:   choice.Message.Content,
					Images:    userImages(choice.Message),
					CreatedAt: eventTime(source),
				})
			}
			continue
		}

		if source.IsError() {
			flushActive()
			timeline.Blocks = append(timeline.Blocks, ErrorBlock{
				Type:      BlockTypeError,
				ID:        eventID(source, "error"),
				Message:   errorMessage(source),
				CreatedAt: eventTime(source),
			})
			continue
		}

		if active == nil {
			active = newBuilder(eventID(source, "history"), eventTime(source))
		}
		active.Add(source)
	}
	flushActive()
	return timeline
}

func userImages(message model.Message) []UserImage {
	result := []UserImage{}
	for _, part := range message.ContentParts {
		if part.Image == nil {
			continue
		}
		imageID, ok := images.ImageIDFromReference(part.Image.URL)
		if !ok {
			continue
		}
		result = append(result, UserImage{ID: imageID})
	}
	return result
}

func eventID(source *event.Event, fallback string) string {
	if source.ID != "" {
		return source.ID
	}
	if source.RequestID != "" {
		return source.RequestID + "_" + fallback
	}
	return fallback
}

func eventTime(source *event.Event) time.Time {
	if !source.Timestamp.IsZero() {
		return source.Timestamp
	}
	return time.Now()
}

func errorMessage(source *event.Event) string {
	if source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return fmt.Sprintf("Agent 运行失败（事件 %s）", eventID(source, "unknown"))
}
