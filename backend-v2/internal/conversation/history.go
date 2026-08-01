package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"edith/backend-v2/internal/usage"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
)

type history struct {
	appName  string
	sessions frameworksession.Service
	usage    *usage.Reader
}

// List 返回用户全部会话的侧边栏摘要。
func (h *history) List(ctx context.Context, userID string) ([]Conversation, error) {
	sessions, err := h.sessions.ListSessions(ctx, frameworksession.UserKey{AppName: h.appName, UserID: strings.TrimSpace(userID)})
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	result := make([]Conversation, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, Conversation{ID: session.ID, Title: title(session.GetEvents()), UpdatedAt: session.UpdatedAt.Format(time.RFC3339)})
	}
	return result, nil
}

// Get 返回一个会话的 Timeline 和服务器计算用量。
func (h *history) Get(ctx context.Context, userID, sessionID string) (ConversationResponse, error) {
	userID, sessionID = strings.TrimSpace(userID), strings.TrimSpace(sessionID)
	session, err := h.sessions.GetSession(ctx, frameworksession.Key{AppName: h.appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return ConversationResponse{}, fmt.Errorf("get conversation: %w", err)
	}
	if session == nil {
		return ConversationResponse{}, errConversationNotFound
	}
	usageSummary, err := h.usage.SessionSummary(ctx, userID, sessionID)
	if err != nil {
		return ConversationResponse{}, err
	}
	return ConversationResponse{Timeline: timeline(session.GetEvents()), Usage: usageSummary}, nil
}

var errConversationNotFound = fmt.Errorf("conversation not found")

func title(events []event.Event) string {
	for index := range events {
		if events[index].Response == nil || !events[index].Response.IsUserMessage() {
			continue
		}
		for _, choice := range events[index].Response.Choices {
			if choice.Message.Role != model.RoleUser {
				continue
			}
			content := strings.TrimSpace(choice.Message.Content)
			if content != "" {
				return truncate(content, 18)
			}
		}
	}
	return "新对话"
}

func truncate(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}
