package conversation

import (
	"strconv"
	"time"

	"edith/backend-v2/internal/agentstream"
	"edith/backend-v2/internal/images"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// timelineProjection 保存一次历史投影正在组装的回复状态。
type timelineProjection struct {
	timeline   Timeline
	assistant  *AssistantBlock
	decoder    *agentstream.Decoder
	toolBlocks map[string]int
}

func timeline(events []event.Event) Timeline {
	projection := newTimelineProjection()
	for index := range events {
		projection.appendFrameworkEvent(&events[index], index)
	}
	projection.finishAssistant()
	return projection.timeline
}

func newTimelineProjection() *timelineProjection {
	return &timelineProjection{
		timeline:   Timeline{Blocks: []TimelineBlock{}},
		toolBlocks: make(map[string]int),
	}
}

// appendFrameworkEvent 把一条框架历史事件放进当前 Timeline。
func (p *timelineProjection) appendFrameworkEvent(source *event.Event, index int) {
	if source.Response != nil && source.Response.IsUserMessage() {
		p.finishAssistant()
		p.appendUserMessages(source, index)
		return
	}
	if source.IsError() {
		p.finishAssistant()
		p.timeline.Blocks = append(p.timeline.Blocks, ErrorBlock{
			Type:      "error",
			ID:        stableEventID(source, "error", index),
			Message:   historyError(source),
			CreatedAt: eventTime(source),
		})
		return
	}
	if source.Response == nil || source.IsRunnerCompletion() {
		return
	}
	if p.assistant == nil {
		assistantID := "assistant_" + stableEventID(source, "history", index)
		p.assistant = &AssistantBlock{
			Type:      "assistant",
			ID:        assistantID,
			CreatedAt: eventTime(source),
			Blocks:    []AssistantContentBlock{},
		}
		p.decoder = agentstream.NewDecoder(assistantID)
	}
	for _, neutral := range p.decoder.DecodeFrameworkEvent(source).Events {
		p.appendNeutralEvent(neutral)
	}
}

func (p *timelineProjection) appendUserMessages(source *event.Event, index int) {
	for _, choice := range source.Response.Choices {
		if choice.Message.Role != model.RoleUser {
			continue
		}
		p.timeline.Blocks = append(p.timeline.Blocks, UserBlock{
			Type:      "user",
			ID:        stableEventID(source, "user", index),
			Content:   choice.Message.Content,
			Images:    userImages(choice.Message),
			CreatedAt: eventTime(source),
		})
	}
}

func (p *timelineProjection) appendNeutralEvent(source agentstream.Event) {
	switch source.Type {
	case "reasoning.delta", "message.delta":
		position := len(p.assistant.Blocks) - 1
		if position < 0 || p.assistant.Blocks[position].ID != source.BlockID {
			p.assistant.Blocks = append(p.assistant.Blocks, AssistantContentBlock{
				Type: source.BlockType,
				ID:   source.BlockID,
			})
			position = len(p.assistant.Blocks) - 1
		}
		p.assistant.Blocks[position].Content += source.Delta
	case "tool.started":
		if _, exists := p.toolBlocks[source.ToolCallID]; exists {
			return
		}
		p.assistant.Blocks = append(p.assistant.Blocks, AssistantContentBlock{
			Type:      "tool",
			ID:        source.ToolCallID,
			ToolName:  source.ToolName,
			Arguments: source.Arguments,
			Status:    "running",
		})
		p.toolBlocks[source.ToolCallID] = len(p.assistant.Blocks) - 1
	case "tool.finished":
		position, exists := p.toolBlocks[source.ToolCallID]
		if !exists {
			return
		}
		p.assistant.Blocks[position].Status = source.ToolStatus
		p.assistant.Blocks[position].Result = source.ToolResult
	}
}

func (p *timelineProjection) finishAssistant() {
	if p.assistant != nil && len(p.assistant.Blocks) > 0 {
		p.timeline.Blocks = append(p.timeline.Blocks, *p.assistant)
	}
	p.assistant = nil
	p.decoder = nil
	p.toolBlocks = make(map[string]int)
}

func userImages(message model.Message) []UserImage {
	result := []UserImage{}
	for _, part := range message.ContentParts {
		if part.Image == nil {
			continue
		}
		imageID, ok := images.ImageIDFromReference(part.Image.URL)
		if ok {
			result = append(result, UserImage{ID: imageID})
		}
	}
	return result
}

func stableEventID(source *event.Event, fallback string, index int) string {
	if source.ID != "" {
		return source.ID
	}
	if source.RequestID != "" {
		return source.RequestID + "_" + fallback
	}
	return fallback + "_" + strconv.Itoa(index)
}

func eventTime(source *event.Event) time.Time {
	if !source.Timestamp.IsZero() {
		return source.Timestamp
	}
	return time.Now()
}

func historyError(source *event.Event) string {
	if source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return "Agent 运行失败"
}
