package webapi

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"edith/backend-v1/internal/usage"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func (s Server) listConversations(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	sessions, err := s.Sessions.ListSessions(r.Context(), session.UserKey{
		AppName: s.AppName,
		UserID:  userID,
	})
	if err != nil {
		http.Error(w, "list conversations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := ConversationListResponse{Conversations: []Conversation{}}
	for _, item := range sessions {
		response.Conversations = append(response.Conversations, Conversation{
			ID:        item.ID,
			Title:     conversationTitle(item.GetEvents()),
			UpdatedAt: item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, response)
}

func (s Server) getConversation(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if userID == "" || sessionID == "" {
		http.Error(w, "userId and sessionID are required", http.StatusBadRequest)
		return
	}

	sess, err := s.Sessions.GetSession(r.Context(), session.Key{
		AppName:   s.AppName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		http.Error(w, "get conversation: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if sess == nil {
		http.Error(w, "conversation not found", http.StatusNotFound)
		return
	}
	if s.UsageDB == nil {
		http.Error(w, "usage service is unavailable", http.StatusServiceUnavailable)
		return
	}
	usageSummary, err := usage.SessionSummary(s.UsageDB, r.Context(), userID, sessionID)
	if err != nil {
		http.Error(w, "summarize conversation usage: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, ConversationResponse{Timeline: buildHistory(sess.GetEvents()), Usage: usageSummary})
}

func conversationTitle(events []event.Event) string {
	for _, item := range events {
		if item.Response == nil || !item.Response.IsUserMessage() {
			continue
		}
		for _, choice := range item.Response.Choices {
			if choice.Message.Role != model.RoleUser {
				continue
			}
			content := strings.TrimSpace(choice.Message.Content)
			if content != "" {
				return truncateRunes(content, 18)
			}
		}
	}
	return "新对话"
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
