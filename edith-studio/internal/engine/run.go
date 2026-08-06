package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"edith/studio/internal/models"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const maxRunDuration = 60 * time.Minute

// Run 同步消费框架事件，并通过 send 逐条交给 Studio。
// 浏览器断开后仍会消费完整事件流，保证 Runner 正常收尾。
func (e *Engine) Run(ctx context.Context, input RunInput, send func(StreamEvent) error) error {
	// 先完成输入检查和 Session 准入，避免同一会话并行修改历史。
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Message = strings.TrimSpace(input.Message)
	if err := ValidateInput(input); err != nil {
		return err
	}
	runOptions, err := e.modelRunOptions(input)
	if err != nil {
		return err
	}
	if err := e.reserveSession(input); err != nil {
		return err
	}
	defer e.releaseSession(input.SessionID)
	defer e.clearUserCanceled(input.RequestID)

	// Runner 使用应用级 ctx；浏览器断开不会取消正在执行的任务。
	runOptions = append(runOptions,
		agent.WithRequestID(input.RequestID),
		agent.WithStream(true),
		agent.WithMaxRunDuration(maxRunDuration),
	)
	frameworkEventCh, err := e.runner.Run(
		ctx,
		e.workspaceID,
		input.SessionID,
		model.NewUserMessage(input.Message),
		runOptions...,
	)
	if err != nil {
		return fmt.Errorf("start agent run: %w", err)
	}

	clientConnected := true
	sendEvent := func(streamEvent StreamEvent) {
		if clientConnected && send(streamEvent) != nil {
			// 浏览器已经断开；继续消费框架事件，让 Runner 完成持久化和资源释放。
			clientConnected = false
		}
	}
	sendEvent(StreamEvent{Type: "run.started"})

	// 这些状态只属于本次回复，用于保持内容块顺序并配对工具结果。
	var lastBlockType string
	var lastBlockID string
	var nextBlockNumber int
	var sawPartialReasoning bool
	var sawPartialText bool
	toolNames := make(map[string]string)
	startedTools := make(map[string]bool)
	failed := false
	completed := false

	// 直接消费框架事件，不创建第二条 channel 或事件翻译对象。
	for frameworkEvent := range frameworkEventCh {
		if frameworkEvent == nil {
			continue
		}
		if frameworkEvent.IsRunnerCompletion() {
			completed = true
			break
		}

		// 用户主动停止由 Engine 自己记录；其他框架错误才展示给用户。
		eventFailed := frameworkEvent.IsError()
		if eventFailed && !failed && !e.wasUserCanceled(input.RequestID) {
			failed = true
			message := eventErrorMessage(frameworkEvent)
			sendEvent(StreamEvent{Type: "run.error", Error: &StreamError{Message: message}})
		}
		if frameworkEvent.Response == nil {
			continue
		}
		for _, choice := range frameworkEvent.Response.Choices {
			// 流式文本与思考共用一个顺序编号，切换类型就开始新块。
			if choice.Delta.ReasoningContent != "" {
				if lastBlockType != "reasoning" {
					nextBlockNumber++
					lastBlockType = "reasoning"
					lastBlockID = fmt.Sprintf("block-%d", nextBlockNumber)
				}
				sawPartialReasoning = true
				sendEvent(StreamEvent{Type: "reasoning.delta", BlockID: lastBlockID, BlockType: "reasoning", Delta: choice.Delta.ReasoningContent})
			}
			if choice.Delta.Content != "" {
				if lastBlockType != "text" {
					nextBlockNumber++
					lastBlockType = "text"
					lastBlockID = fmt.Sprintf("block-%d", nextBlockNumber)
				}
				sawPartialText = true
				sendEvent(StreamEvent{Type: "message.delta", BlockID: lastBlockID, BlockType: "text", Delta: choice.Delta.Content})
			}
			if choice.Message.ToolID != "" {
				toolCallID := choice.Message.ToolID
				if !startedTools[toolCallID] {
					startedTools[toolCallID] = true
					sendEvent(StreamEvent{
						Type:       "tool.started",
						ToolCallID: toolCallID,
						ToolName:   toolName(choice.Message.ToolName, toolNames[toolCallID]),
						ToolStatus: "running",
					})
				}
				status := "completed"
				if eventFailed {
					status = "failed"
				}
				sendEvent(StreamEvent{
					Type:       "tool.finished",
					ToolCallID: toolCallID,
					ToolName:   toolName(choice.Message.ToolName, toolNames[toolCallID]),
					ToolResult: choice.Message.Content,
					ToolStatus: status,
				})
				lastBlockType = ""
				lastBlockID = ""
				continue
			}

			// 工具调用只读取完整 Response，避免 partial 事件重复创建卡片。
			if frameworkEvent.Response.IsPartial {
				continue
			}
			for _, toolCall := range choice.Message.ToolCalls {
				if toolCall.ID == "" || startedTools[toolCall.ID] {
					continue
				}
				startedTools[toolCall.ID] = true
				toolNames[toolCall.ID] = toolCall.Function.Name
				sendEvent(StreamEvent{
					Type:       "tool.started",
					ToolCallID: toolCall.ID,
					ToolName:   toolCall.Function.Name,
					Arguments:  string(toolCall.Function.Arguments),
					ToolStatus: "running",
				})
				lastBlockType = ""
				lastBlockID = ""
			}

			// 非流式模型把完整回答放在 Message 中；已有 delta 时不再重复发送。
			if len(choice.Message.ToolCalls) == 0 && choice.Message.ReasoningContent != "" && !sawPartialReasoning {
				if lastBlockType != "reasoning" {
					nextBlockNumber++
					lastBlockType = "reasoning"
					lastBlockID = fmt.Sprintf("block-%d", nextBlockNumber)
				}
				sendEvent(StreamEvent{Type: "reasoning.delta", BlockID: lastBlockID, BlockType: "reasoning", Delta: choice.Message.ReasoningContent})
			}
			if len(choice.Message.ToolCalls) == 0 && choice.Message.Content != "" && !sawPartialText {
				if lastBlockType != "text" {
					nextBlockNumber++
					lastBlockType = "text"
					lastBlockID = fmt.Sprintf("block-%d", nextBlockNumber)
				}
				sendEvent(StreamEvent{Type: "message.delta", BlockID: lastBlockID, BlockType: "text", Delta: choice.Message.Content})
			}
		}
		if !frameworkEvent.Response.IsPartial {
			sawPartialReasoning = false
			sawPartialText = false
		}
	}

	// 只有框架 completion 才表示 Runner 已完成持久化和内部收尾。
	if !completed {
		err := errors.New("framework event stream ended without runner completion")
		if !failed {
			sendEvent(StreamEvent{Type: "run.error", Error: &StreamError{Message: err.Error()}})
		}
		return err
	}
	if e.wasUserCanceled(input.RequestID) {
		sendEvent(StreamEvent{Type: "run.canceled"})
	} else if !failed {
		sendEvent(StreamEvent{Type: "run.completed"})
	}
	return nil
}

func (e *Engine) modelRunOptions(input RunInput) ([]agent.RunOption, error) {
	options, err := e.models.RunOptions(models.Selection{
		ModelID:      input.ModelID,
		ThinkingMode: input.ThinkingMode,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve model selection: %w", err)
	}
	return options, nil
}

func eventErrorMessage(frameworkEvent *event.Event) string {
	if frameworkEvent.Response != nil && frameworkEvent.Response.Error != nil {
		return frameworkEvent.Response.Error.Message
	}
	return "Agent run failed"
}

func toolName(first, fallback string) string {
	if strings.TrimSpace(first) != "" {
		return first
	}
	return fallback
}
