package webapi

import (
	"fmt"
	"strconv"
	"time"

	"edith/backend-v1/internal/images"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// buildHistory 把已保存的 Agent 事件直接还原成浏览器需要的完整 Timeline。
func buildHistory(events []event.Event) Timeline {
	result := Timeline{Blocks: []TimelineBlock{}}

	var assistant *AssistantBlock
	toolIndex := map[string]int{}
	nextBlockID := 0
	sawPartialReasoning := false
	sawPartialText := false

	for index := range events {
		source := &events[index]
		if source.Response == nil || source.IsRunnerCompletion() {
			continue
		}

		if source.Response.IsUserMessage() {
			if assistant != nil && len(assistant.Blocks) > 0 {
				result.Blocks = append(result.Blocks, *assistant)
			}
			assistant = nil
			toolIndex = map[string]int{}
			nextBlockID = 0
			sawPartialReasoning = false
			sawPartialText = false

			for _, choice := range source.Response.Choices {
				if choice.Message.Role != model.RoleUser {
					continue
				}

				userImages := []UserImage{}
				for _, part := range choice.Message.ContentParts {
					if part.Image == nil {
						continue
					}
					imageID, ok := images.ImageIDFromReference(part.Image.URL)
					if !ok {
						continue
					}
					userImages = append(userImages, UserImage{ID: imageID})
				}

				result.Blocks = append(result.Blocks, UserBlock{
					Type:      BlockTypeUser,
					ID:        eventID(source, "user"),
					Content:   choice.Message.Content,
					Images:    userImages,
					CreatedAt: eventTime(source),
				})
			}
			continue
		}

		if source.IsError() {
			if assistant != nil && len(assistant.Blocks) > 0 {
				result.Blocks = append(result.Blocks, *assistant)
			}
			assistant = nil
			toolIndex = map[string]int{}
			nextBlockID = 0
			sawPartialReasoning = false
			sawPartialText = false

			result.Blocks = append(result.Blocks, ErrorBlock{
				Type:      BlockTypeError,
				ID:        eventID(source, "error"),
				Message:   errorMessage(source),
				CreatedAt: eventTime(source),
			})
			continue
		}

		if assistant == nil {
			assistant = &AssistantBlock{
				Type:      BlockTypeAssistant,
				ID:        "assistant_" + eventID(source, "history"),
				CreatedAt: eventTime(source),
				Blocks:    []AssistantContentBlock{},
			}
		}

		for _, choice := range source.Response.Choices {
			if choice.Delta.ReasoningContent != "" {
				last := len(assistant.Blocks) - 1
				if last < 0 || assistant.Blocks[last].Type != AssistantContentBlockTypeReasoning {
					nextBlockID++
					assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
						Type: AssistantContentBlockTypeReasoning,
						ID:   assistant.ID + "_reasoning_" + strconv.Itoa(nextBlockID),
					})
					last = len(assistant.Blocks) - 1
				}
				assistant.Blocks[last].Content += choice.Delta.ReasoningContent
				sawPartialReasoning = true
			}

			if choice.Delta.Content != "" {
				last := len(assistant.Blocks) - 1
				if last < 0 || assistant.Blocks[last].Type != AssistantContentBlockTypeText {
					nextBlockID++
					assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
						Type: AssistantContentBlockTypeText,
						ID:   assistant.ID + "_text_" + strconv.Itoa(nextBlockID),
					})
					last = len(assistant.Blocks) - 1
				}
				assistant.Blocks[last].Content += choice.Delta.Content
				sawPartialText = true
			}

			if choice.Message.ToolID != "" {
				toolPosition, started := toolIndex[choice.Message.ToolID]
				if !started {
					assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
						Type:     AssistantContentBlockTypeTool,
						ID:       choice.Message.ToolID,
						ToolName: choice.Message.ToolName,
						Status:   ToolStatusRunning,
					})
					toolPosition = len(assistant.Blocks) - 1
					toolIndex[choice.Message.ToolID] = toolPosition
				}

				status := ToolStatusCompleted
				if source.Error != nil {
					status = ToolStatusFailed
				}
				assistant.Blocks[toolPosition].Status = status
				assistant.Blocks[toolPosition].Result = choice.Message.Content
				continue
			}

			if source.Response.IsPartial {
				continue
			}

			if choice.Message.ReasoningContent != "" && !sawPartialReasoning {
				last := len(assistant.Blocks) - 1
				if last < 0 || assistant.Blocks[last].Type != AssistantContentBlockTypeReasoning {
					nextBlockID++
					assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
						Type: AssistantContentBlockTypeReasoning,
						ID:   assistant.ID + "_reasoning_" + strconv.Itoa(nextBlockID),
					})
					last = len(assistant.Blocks) - 1
				}
				assistant.Blocks[last].Content += choice.Message.ReasoningContent
			}

			if choice.Message.Content != "" && !sawPartialText {
				last := len(assistant.Blocks) - 1
				if last < 0 || assistant.Blocks[last].Type != AssistantContentBlockTypeText {
					nextBlockID++
					assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
						Type: AssistantContentBlockTypeText,
						ID:   assistant.ID + "_text_" + strconv.Itoa(nextBlockID),
					})
					last = len(assistant.Blocks) - 1
				}
				assistant.Blocks[last].Content += choice.Message.Content
			}

			for _, call := range choice.Message.ToolCalls {
				if call.ID == "" {
					continue
				}
				if _, started := toolIndex[call.ID]; started {
					continue
				}

				assistant.Blocks = append(assistant.Blocks, AssistantContentBlock{
					Type:      AssistantContentBlockTypeTool,
					ID:        call.ID,
					ToolName:  call.Function.Name,
					Arguments: string(call.Function.Arguments),
					Status:    ToolStatusRunning,
				})
				toolIndex[call.ID] = len(assistant.Blocks) - 1
			}
		}

		if !source.Response.IsPartial {
			sawPartialReasoning = false
			sawPartialText = false
		}
	}

	if assistant != nil && len(assistant.Blocks) > 0 {
		result.Blocks = append(result.Blocks, *assistant)
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
