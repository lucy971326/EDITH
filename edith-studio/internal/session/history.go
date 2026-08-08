package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
)

const (
	studioAppName   = "edith-studio"
	maxTitleRunes   = 36
	untitledSession = "新会话"
)

// List 返回指定 Workspace 的全部会话摘要，按框架的更新时间倒序排列。
func (m *Module) List(ctx context.Context, workspaceID string) ([]Summary, error) {
	sessions, err := m.service.ListSessions(ctx, frameworksession.UserKey{
		AppName: studioAppName,
		UserID:  workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	summaries := make([]Summary, 0, len(sessions))
	for _, storedSession := range sessions {
		if storedSession == nil {
			continue
		}
		summaries = append(summaries, summaryFromSession(storedSession))
	}
	return summaries, nil
}

// Get 读取指定 Workspace 中一个会话，并还原为聊天时间线数据。
func (m *Module) Get(ctx context.Context, workspaceID, sessionID string) (History, error) {
	key, err := sessionKey(workspaceID, sessionID)
	if err != nil {
		return History{}, err
	}
	storedSession, err := m.service.GetSession(ctx, key)
	if err != nil {
		return History{}, fmt.Errorf("get session: %w", err)
	}
	if storedSession == nil {
		return History{}, ErrSessionNotFound
	}
	return History{
		Session:  summaryFromSession(storedSession),
		Messages: messagesFromEvents(storedSession.GetEvents()),
	}, nil
}

// Delete 删除指定 Workspace 中一个已存在的会话。
func (m *Module) Delete(ctx context.Context, workspaceID, sessionID string) error {
	key, err := sessionKey(workspaceID, sessionID)
	if err != nil {
		return err
	}
	storedSession, err := m.service.GetSession(ctx, key)
	if err != nil {
		return fmt.Errorf("get session before deletion: %w", err)
	}
	if storedSession == nil {
		return ErrSessionNotFound
	}
	if err := m.service.DeleteSession(ctx, key); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func sessionKey(workspaceID, sessionID string) (frameworksession.Key, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || strings.ContainsAny(sessionID, `/\\`) {
		return frameworksession.Key{}, ErrInvalidSessionID
	}
	return frameworksession.Key{AppName: studioAppName, UserID: workspaceID, SessionID: sessionID}, nil
}

// SessionKey 返回当前 Workspace 使用的框架 Session 身份。
// Engine 通过它读取和压缩同一条会话，避免重复定义 AppName。
func SessionKey(workspaceID, sessionID string) (frameworksession.Key, error) {
	return sessionKey(workspaceID, sessionID)
}

func summaryFromSession(storedSession *frameworksession.Session) Summary {
	return Summary{
		ID:        storedSession.ID,
		Title:     sessionTitle(storedSession.GetEvents()),
		UpdatedAt: storedSession.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func sessionTitle(events []event.Event) string {
	for _, frameworkEvent := range events {
		if !frameworkEvent.IsUserMessage() || frameworkEvent.Response == nil {
			continue
		}
		for _, choice := range frameworkEvent.Response.Choices {
			if strings.TrimSpace(choice.Message.Content) == "" {
				continue
			}
			content := choice.Message.Content
			if utf8.RuneCountInString(content) <= maxTitleRunes {
				return content
			}
			return string([]rune(content)[:maxTitleRunes]) + "…"
		}
	}
	return untitledSession
}

func messagesFromEvents(events []event.Event) []ChatMessage {
	// 预扫描：收集所有触发过子事件（AgentTool 调用）的父工具调用 ID。
	// 这些调用在还原时表现为 agent 块；其余工具仍还原为普通工具块。
	agentCallIDs := make(map[string]bool)
	for _, frameworkEvent := range events {
		if frameworkEvent.ParentMetadata != nil && frameworkEvent.ParentMetadata.TriggerID != "" {
			agentCallIDs[frameworkEvent.ParentMetadata.TriggerID] = true
		}
	}

	messages := make([]ChatMessage, 0)
	var assistant *ChatMessage
	// currentAgentIndex 是当前打开的 agent 块在 assistant.Blocks 中的下标；-1 表示不在子 Agent 内。
	// 用下标而非指针，避免 slice 扩容后指针失效。
	currentAgentIndex := -1

	appendAssistant := func() {
		if assistant == nil {
			return
		}
		if len(assistant.Blocks) > 0 {
			messages = append(messages, *assistant)
		}
		assistant = nil
	}

	for _, frameworkEvent := range events {
		if frameworkEvent.Response == nil {
			continue
		}
		if frameworkEvent.IsUserMessage() {
			// 用户消息结束当前子 Agent 上下文。
			currentAgentIndex = -1
			appendAssistant()
			for choiceIndex, choice := range frameworkEvent.Response.Choices {
				if strings.TrimSpace(choice.Message.Content) == "" && len(imageDataURLs(choice.Message)) == 0 {
					continue
				}
				messages = append(messages, ChatMessage{
					ID:      messageID(frameworkEvent.ID, choiceIndex),
					Role:    "user",
					Content: choice.Message.Content,
					Images:  imageDataURLs(choice.Message),
				})
			}
			continue
		}

		if assistant == nil {
			assistant = &ChatMessage{ID: frameworkEvent.ID, Role: "assistant"}
		}

		// 子事件（ParentMetadata 非 nil）：内容归入当前 agent 块的 Blocks。
		if frameworkEvent.ParentMetadata != nil {
			if currentAgentIndex >= 0 {
				agentBlock := &assistant.Blocks[currentAgentIndex]
				for choiceIndex, choice := range frameworkEvent.Response.Choices {
					appendChoiceBlocks(&agentBlock.Blocks, frameworkEvent, choice, choiceIndex, agentCallIDs)
				}
			}
			continue
		}

		for choiceIndex, choice := range frameworkEvent.Response.Choices {
			message := choice.Message
			// 父收到 agent 工具结果：关闭 agent 块并填 result。
			if message.ToolID != "" && currentAgentIndex >= 0 && message.ToolID == assistant.Blocks[currentAgentIndex].ID {
				agentBlock := &assistant.Blocks[currentAgentIndex]
				agentBlock.Result = message.Content
				if frameworkEvent.IsError() {
					agentBlock.Status = "failed"
				} else {
					agentBlock.Status = "completed"
				}
				currentAgentIndex = -1
				continue
			}
			// 先追加父的思考/文本/普通工具，再打开 agent 块：
			// 保证父思考块出现在 agent 卡之前，与实时流式顺序一致。
			appendChoiceBlocks(&assistant.Blocks, frameworkEvent, choice, choiceIndex, agentCallIDs)
			// 父发起的 AgentTool 调用：打开 agent 块（appendChoiceBlocks 已跳过这些调用）。
			for _, toolCall := range message.ToolCalls {
				if agentCallIDs[toolCall.ID] {
					assistant.Blocks = append(assistant.Blocks, AssistantBlock{
						ID:        toolCall.ID,
						Type:      "agent",
						Name:      toolCall.Function.Name,
						Arguments: string(toolCall.Function.Arguments),
						Status:    "running",
					})
					currentAgentIndex = len(assistant.Blocks) - 1
				}
			}
		}
		if frameworkEvent.IsError() && frameworkEvent.Response.Error != nil {
			assistant.Blocks = append(assistant.Blocks, AssistantBlock{
				ID:      messageID(frameworkEvent.ID, len(assistant.Blocks)),
				Type:    "error",
				Content: frameworkEvent.Response.Error.Message,
			})
		}
	}
	appendAssistant()
	return messages
}

// imageDataURLs 把用户消息中的图片内容块还原为前端可显示的 data URL。
func imageDataURLs(message model.Message) []ChatImage {
	images := make([]ChatImage, 0)
	for _, part := range message.ContentParts {
		if part.Type != model.ContentTypeImage || part.Image == nil || len(part.Image.Data) == 0 {
			continue
		}
		format := part.Image.Format
		if format == "" {
			format = "png"
		}
		images = append(images, ChatImage{
			DataURL: "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(part.Image.Data),
		})
	}
	return images
}

// appendChoiceBlocks 把一条 choice 的思考/文本/工具调用/工具结果追加到任意 blocks 切片。
// agentCallIDs 标记的 AgentTool 调用由调用方单独打开 agent 块，这里跳过，避免重复。
func appendChoiceBlocks(blocks *[]AssistantBlock, frameworkEvent event.Event, choice model.Choice, choiceIndex int, agentCallIDs map[string]bool) {
	message := choice.Message
	if message.ReasoningContent != "" {
		*blocks = append(*blocks, AssistantBlock{
			ID:      messageID(frameworkEvent.ID, choiceIndex),
			Type:    "reasoning",
			Content: message.ReasoningContent,
		})
	}
	if message.Content != "" && message.ToolID == "" {
		*blocks = append(*blocks, AssistantBlock{
			ID:      messageID(frameworkEvent.ID, choiceIndex+len(*blocks)),
			Type:    "text",
			Content: message.Content,
		})
	}
	for toolIndex, toolCall := range message.ToolCalls {
		if agentCallIDs[toolCall.ID] {
			continue
		}
		block := AssistantBlock{
			ID:        toolCall.ID,
			Type:      "tool",
			Name:      toolCall.Function.Name,
			Arguments: string(toolCall.Function.Arguments),
			Status:    "requested",
		}
		if toolCall.ID == "" {
			block.ID = messageID(frameworkEvent.ID, choiceIndex+toolIndex)
		}
		*blocks = append(*blocks, block)
	}
	if message.ToolID != "" {
		appendToolResultTo(blocks, message, frameworkEvent.IsError())
	}
}

// appendToolResultTo 把工具结果回填到 blocks 中匹配的 tool 块；找不到时追加新块。
func appendToolResultTo(blocks *[]AssistantBlock, message model.Message, failed bool) {
	for index := range *blocks {
		block := &(*blocks)[index]
		if block.Type != "tool" || block.ID != message.ToolID {
			continue
		}
		block.Result = message.Content
		if failed {
			block.Status = "failed"
		}
		return
	}
	status := "completed"
	if failed {
		status = "failed"
	}
	*blocks = append(*blocks, AssistantBlock{
		ID:     message.ToolID,
		Type:   "tool",
		Name:   message.ToolName,
		Result: message.Content,
		Status: status,
	})
}

func messageID(eventID string, index int) string {
	return fmt.Sprintf("%s-%d", eventID, index)
}
