package session

import (
	"context"
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
	messages := make([]ChatMessage, 0)
	var assistant *ChatMessage

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
			appendAssistant()
			for choiceIndex, choice := range frameworkEvent.Response.Choices {
				if strings.TrimSpace(choice.Message.Content) == "" {
					continue
				}
				messages = append(messages, ChatMessage{
					ID:      messageID(frameworkEvent.ID, choiceIndex),
					Role:    "user",
					Content: choice.Message.Content,
				})
			}
			continue
		}

		if assistant == nil {
			assistant = &ChatMessage{ID: frameworkEvent.ID, Role: "assistant"}
		}
		for choiceIndex, choice := range frameworkEvent.Response.Choices {
			appendAssistantChoice(assistant, frameworkEvent, choice, choiceIndex)
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

func appendAssistantChoice(assistant *ChatMessage, frameworkEvent event.Event, choice model.Choice, choiceIndex int) {
	message := choice.Message
	if message.ReasoningContent != "" {
		assistant.Blocks = append(assistant.Blocks, AssistantBlock{
			ID:      messageID(frameworkEvent.ID, choiceIndex),
			Type:    "reasoning",
			Content: message.ReasoningContent,
		})
	}
	if message.Content != "" && message.ToolID == "" {
		assistant.Blocks = append(assistant.Blocks, AssistantBlock{
			ID:      messageID(frameworkEvent.ID, choiceIndex+len(assistant.Blocks)),
			Type:    "text",
			Content: message.Content,
		})
	}
	for toolIndex, toolCall := range message.ToolCalls {
		assistant.Blocks = append(assistant.Blocks, AssistantBlock{
			ID:        toolCall.ID,
			Type:      "tool",
			Name:      toolCall.Function.Name,
			Arguments: string(toolCall.Function.Arguments),
			Status:    "requested",
		})
		if toolCall.ID == "" {
			assistant.Blocks[len(assistant.Blocks)-1].ID = messageID(frameworkEvent.ID, choiceIndex+toolIndex)
		}
	}
	if message.ToolID != "" {
		appendToolResult(assistant, message, frameworkEvent.IsError())
	}
}

func appendToolResult(assistant *ChatMessage, message model.Message, failed bool) {
	for index := range assistant.Blocks {
		block := &assistant.Blocks[index]
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
	assistant.Blocks = append(assistant.Blocks, AssistantBlock{
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
