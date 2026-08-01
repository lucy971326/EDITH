package agentstream

import (
	"strconv"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// Decoder 保存一次 Agent 回复中的块顺序；每个事件流必须使用独立实例。
type Decoder struct {
	assistantID         string
	nextBlockID         int
	lastType            string
	lastID              string
	sawPartialReasoning bool
	sawPartialText      bool
	latestUsage         *model.Usage
	startedTools        map[string]bool
}

// NewDecoder 为一次回复创建事件解释器。
func NewDecoder(assistantID string) *Decoder {
	return &Decoder{
		assistantID:  assistantID,
		startedTools: make(map[string]bool),
	}
}

// DecodeFrameworkEvent 把一个框架事件解释为零个或多个中性事件。
func (d *Decoder) DecodeFrameworkEvent(source *event.Event) FrameworkEventResult {
	result := FrameworkEventResult{}
	if source == nil {
		return result
	}
	if source.IsError() {
		result.ErrorMessage = frameworkErrorMessage(source)
	}
	if source.IsRunnerCompletion() {
		result.Completed = true
		result.Usage = d.takeLatestUsage()
		return result
	}
	if source.Response == nil {
		return result
	}
	if source.Response.Usage != nil {
		d.latestUsage = source.Response.Usage
	}

	for _, choice := range source.Response.Choices {
		if choice.Delta.ReasoningContent != "" {
			result.Events = append(result.Events, d.contentDelta("reasoning", "reasoning.delta", choice.Delta.ReasoningContent))
			d.sawPartialReasoning = true
		}
		if choice.Delta.Content != "" {
			result.Events = append(result.Events, d.contentDelta("text", "message.delta", choice.Delta.Content))
			d.sawPartialText = true
		}
		if choice.Message.ToolID != "" {
			result.Events = append(result.Events, d.toolResult(source, choice.Message.ToolID, choice.Message.ToolName, choice.Message.Content)...)
			continue
		}
		if source.Response.IsPartial {
			continue
		}
		toolStarted := false
		for _, call := range choice.Message.ToolCalls {
			if call.ID == "" || d.startedTools[call.ID] {
				continue
			}
			d.startedTools[call.ID] = true
			toolStarted = true
			result.Events = append(result.Events, Event{
				Type: "tool.started", AssistantID: d.assistantID,
				ToolCallID: call.ID, ToolName: call.Function.Name,
				Arguments: string(call.Function.Arguments), ToolStatus: "running",
			})
		}
		if toolStarted {
			d.endContentBlock()
		}
		if choice.Message.ToolID == "" && len(choice.Message.ToolCalls) == 0 {
			if choice.Delta.ReasoningContent == "" &&
				choice.Message.ReasoningContent != "" &&
				!d.sawPartialReasoning {
				result.Events = append(result.Events, d.contentDelta("reasoning", "reasoning.delta", choice.Message.ReasoningContent))
			}
			if choice.Delta.Content == "" &&
				choice.Message.Content != "" &&
				!d.sawPartialText {
				result.Events = append(result.Events, d.contentDelta("text", "message.delta", choice.Message.Content))
			}
		}
	}
	if !source.Response.IsPartial {
		result.Usage = d.takeLatestUsage()
		d.sawPartialReasoning = false
		d.sawPartialText = false
	}
	return result
}

func (d *Decoder) takeLatestUsage() *model.Usage {
	latest := d.latestUsage
	d.latestUsage = nil
	return latest
}

func (d *Decoder) contentDelta(blockType, eventType, delta string) Event {
	if d.lastType != blockType {
		d.nextBlockID++
		d.lastType = blockType
		d.lastID = d.assistantID + "_" + blockType + "_" + strconv.Itoa(d.nextBlockID)
	}
	return Event{
		Type: eventType, AssistantID: d.assistantID,
		BlockID: d.lastID, BlockType: blockType, Delta: delta,
	}
}

func (d *Decoder) toolResult(source *event.Event, toolID, toolName, content string) []Event {
	events := make([]Event, 0, 2)
	if !d.startedTools[toolID] {
		d.startedTools[toolID] = true
		events = append(events, Event{
			Type: "tool.started", AssistantID: d.assistantID,
			ToolCallID: toolID, ToolName: toolName, ToolStatus: "running",
		})
	}
	status := "completed"
	if source.Error != nil {
		status = "failed"
	}
	events = append(events, Event{
		Type: "tool.finished", AssistantID: d.assistantID,
		ToolCallID: toolID, ToolStatus: status, ToolResult: content,
	})
	d.endContentBlock()
	return events
}

func (d *Decoder) endContentBlock() {
	d.lastType = ""
	d.lastID = ""
}

func frameworkErrorMessage(source *event.Event) string {
	if source.Error != nil && source.Error.Message != "" {
		return source.Error.Message
	}
	return "agent runner returned an error"
}
