package timeline

import (
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestBuilderStreamSequence(t *testing.T) {
	builder := NewBuilder("request-1")
	if started := builder.Started(); started.Assistant.ID != "assistant_request-1" {
		t.Fatalf("Started().Assistant.ID = %q", started.Assistant.ID)
	}

	firstDelta := responseEvent(model.Response{
		IsPartial: true,
		Choices: []model.Choice{{
			Delta: model.Message{
				ReasoningContent: "用户问我几点了。",
				Content:          "你好！我帮你看看！",
			},
		}},
	})
	if got := builder.Add(firstDelta); len(got) != 2 {
		t.Fatalf("first delta stream events = %d, want 2", len(got))
	}

	toolCall := model.ToolCall{
		ID: "call_time",
		Function: model.FunctionDefinitionParam{
			Name:      "get_current_time",
			Arguments: []byte(`{"timezone":"Asia/Shanghai"}`),
		},
	}
	toolCallEvent := responseEvent(model.Response{
		Done: true,
		Choices: []model.Choice{{
			Message: model.Message{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{toolCall}},
		}},
	})
	if got := builder.Add(toolCallEvent); len(got) != 1 {
		t.Fatalf("tool call stream events = %d, want 1", len(got))
	}

	toolResult := responseEvent(model.Response{
		Object: model.ObjectTypeToolResponse,
		Choices: []model.Choice{{
			Message: model.Message{
				Role:     model.RoleTool,
				ToolID:   "call_time",
				ToolName: "get_current_time",
				Content:  `{"time":"10:00"}`,
			},
		}},
	})
	if got := builder.Add(toolResult); len(got) != 1 {
		t.Fatalf("tool result stream events = %d, want 1", len(got))
	}

	secondDelta := responseEvent(model.Response{
		IsPartial: true,
		Choices: []model.Choice{{
			Delta: model.Message{
				ReasoningContent: "现在十点了。",
				Content:          "哈哈，现在十点了！",
			},
		}},
	})
	builder.Add(secondDelta)

	// A streaming provider also emits a complete final message. The builder
	// must not duplicate text that was already received in Delta chunks.
	builder.Add(responseEvent(model.Response{
		Done: true,
		Choices: []model.Choice{{
			Message: model.Message{
				Role:             model.RoleAssistant,
				ReasoningContent: "现在十点了。",
				Content:          "哈哈，现在十点了！",
			},
		}},
	}))

	blocks := builder.Assistant().Blocks
	if len(blocks) != 5 {
		t.Fatalf("assistant blocks = %d, want 5: %#v", len(blocks), blocks)
	}
	assertContentBlock(t, blocks[0], AssistantContentBlockTypeReasoning, "用户问我几点了。")
	assertContentBlock(t, blocks[1], AssistantContentBlockTypeText, "你好！我帮你看看！")
	if blocks[2].Type != AssistantContentBlockTypeTool ||
		blocks[2].ID != "call_time" ||
		blocks[2].Status != ToolStatusCompleted ||
		blocks[2].Result != `{"time":"10:00"}` {
		t.Fatalf("tool block = %#v", blocks[2])
	}
	assertContentBlock(t, blocks[3], AssistantContentBlockTypeReasoning, "现在十点了。")
	assertContentBlock(t, blocks[4], AssistantContentBlockTypeText, "哈哈，现在十点了！")
}

func TestBuildHistory(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	events := []event.Event{
		{
			ID:        "user-1",
			Timestamp: now,
			Response: &model.Response{Choices: []model.Choice{{
				Message: model.Message{Role: model.RoleUser, Content: "你好，看看几点了"},
			}}},
		},
		{
			ID:        "assistant-1",
			Timestamp: now.Add(time.Second),
			Response: &model.Response{Choices: []model.Choice{{
				Message: model.Message{
					Role:             model.RoleAssistant,
					ReasoningContent: "用户问我几点了。",
					Content:          "你好！我帮你看看！",
					ToolCalls: []model.ToolCall{{
						ID: "call_time",
						Function: model.FunctionDefinitionParam{
							Name:      "get_current_time",
							Arguments: []byte(`{"timezone":"Asia/Shanghai"}`),
						},
					}},
				},
			}}},
		},
		{
			ID:        "tool-1",
			Timestamp: now.Add(2 * time.Second),
			Response: &model.Response{Object: model.ObjectTypeToolResponse, Choices: []model.Choice{{
				Message: model.Message{
					Role:     model.RoleTool,
					ToolID:   "call_time",
					ToolName: "get_current_time",
					Content:  `{"time":"10:00"}`,
				},
			}}},
		},
		{
			ID:        "assistant-2",
			Timestamp: now.Add(3 * time.Second),
			Response: &model.Response{Choices: []model.Choice{{
				Message: model.Message{
					Role:             model.RoleAssistant,
					ReasoningContent: "现在十点了。",
					Content:          "哈哈，现在十点了！",
				},
			}}},
		},
	}

	timeline := BuildHistory(events)
	if len(timeline.Blocks) != 2 {
		t.Fatalf("timeline blocks = %d, want 2", len(timeline.Blocks))
	}
	user, ok := timeline.Blocks[0].(UserBlock)
	if !ok || user.Content != "你好，看看几点了" {
		t.Fatalf("user block = %#v", timeline.Blocks[0])
	}
	assistant, ok := timeline.Blocks[1].(AssistantBlock)
	if !ok {
		t.Fatalf("assistant block = %#v", timeline.Blocks[1])
	}
	if len(assistant.Blocks) != 5 {
		t.Fatalf("assistant blocks = %d, want 5", len(assistant.Blocks))
	}
	assertContentBlock(t, assistant.Blocks[0], AssistantContentBlockTypeReasoning, "用户问我几点了。")
	assertContentBlock(t, assistant.Blocks[1], AssistantContentBlockTypeText, "你好！我帮你看看！")
	if assistant.Blocks[2].Result != `{"time":"10:00"}` {
		t.Fatalf("tool result = %q", assistant.Blocks[2].Result)
	}
	assertContentBlock(t, assistant.Blocks[3], AssistantContentBlockTypeReasoning, "现在十点了。")
	assertContentBlock(t, assistant.Blocks[4], AssistantContentBlockTypeText, "哈哈，现在十点了！")
}

func responseEvent(response model.Response) *event.Event {
	return &event.Event{Response: &response}
}

func assertContentBlock(
	t *testing.T,
	block AssistantContentBlock,
	wantType AssistantContentBlockType,
	wantContent string,
) {
	t.Helper()
	if block.Type != wantType || block.Content != wantContent {
		t.Fatalf("block = %#v, want type=%q content=%q", block, wantType, wantContent)
	}
}
