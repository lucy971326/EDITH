package agentstream

import (
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestDecoderKeepsContinuousTextInOneBlock(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	first := decoder.DecodeFrameworkEvent(textEvent("你")).Events[0]
	second := decoder.DecodeFrameworkEvent(textEvent("好")).Events[0]

	if first.BlockID == "" || first.BlockID != second.BlockID {
		t.Fatalf("连续文本应共享 BlockID: %q / %q", first.BlockID, second.BlockID)
	}
}

func TestDecoderStartsNewTextBlockAfterTool(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	before := decoder.DecodeFrameworkEvent(textEvent("工具前")).Events[0]
	decoder.DecodeFrameworkEvent(toolCallEvent("tool-1", "create_cron_job"))
	decoder.DecodeFrameworkEvent(toolResultEvent("tool-1", "create_cron_job"))
	after := decoder.DecodeFrameworkEvent(textEvent("工具后")).Events[0]

	if before.BlockID == after.BlockID {
		t.Fatalf("Tool 前后必须是不同文本块: %q", before.BlockID)
	}
}

func TestDecoderReadsCompleteHistoricalMessage(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	result := decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		Choices: []model.Choice{{Message: model.Message{Content: "完整历史回复"}}},
	}})

	if len(result.Events) != 1 || result.Events[0].Delta != "完整历史回复" {
		t.Fatalf("完整消息没有转换为文本事件: %#v", result.Events)
	}
}

func TestDecoderDoesNotRepeatStreamingContentFromFinalMessage(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		IsPartial: true,
		Choices: []model.Choice{{Delta: model.Message{
			ReasoningContent: "正在思考",
			Content:          "实时回复",
		}}},
	}})

	result := decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		Choices: []model.Choice{{Message: model.Message{
			ReasoningContent: "正在思考",
			Content:          "实时回复",
		}}},
	}})

	if len(result.Events) != 0 {
		t.Fatalf("最终完整消息不应重复已经输出的增量内容: %#v", result.Events)
	}
}

func TestDecoderKeepsLastStreamingUsageUntilResponseEnds(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		IsPartial: true,
		Usage:     &model.Usage{TotalTokens: 12},
	}})
	decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		IsPartial: true,
		Usage:     &model.Usage{TotalTokens: 18},
	}})

	result := decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{}})
	if result.Usage == nil || result.Usage.TotalTokens != 18 {
		t.Fatalf("响应结束时应返回最后一个流式 Usage: %#v", result.Usage)
	}
}

func TestDecoderReturnsPendingUsageOnRunnerCompletion(t *testing.T) {
	decoder := NewDecoder("assistant-1")
	decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		IsPartial: true,
		Usage:     &model.Usage{TotalTokens: 21},
	}})

	result := decoder.DecodeFrameworkEvent(&event.Event{Response: &model.Response{
		Done:   true,
		Object: model.ObjectTypeRunnerCompletion,
	}})
	if !result.Completed || result.Usage == nil || result.Usage.TotalTokens != 21 {
		t.Fatalf("Runner 完成时应返回尚未提交的 Usage: %#v", result)
	}
}

func textEvent(content string) *event.Event {
	return &event.Event{Response: &model.Response{
		IsPartial: true,
		Choices:   []model.Choice{{Delta: model.Message{Content: content}}},
	}}
}

func toolCallEvent(toolID, toolName string) *event.Event {
	return &event.Event{Response: &model.Response{Choices: []model.Choice{{Message: model.Message{
		ToolCalls: []model.ToolCall{{
			ID: toolID,
			Function: model.FunctionDefinitionParam{
				Name:      toolName,
				Arguments: []byte(`{"name":"提醒"}`),
			},
		}},
	}}}}}
}

func toolResultEvent(toolID, toolName string) *event.Event {
	return &event.Event{Response: &model.Response{Choices: []model.Choice{{Message: model.Message{
		ToolID: toolID, ToolName: toolName, Content: `{"ok":true}`,
	}}}}}
}
