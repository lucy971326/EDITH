package main

import (
	"encoding/json"
	"net/http"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// ============================================================================
// 会话历史
// ============================================================================

// ChatMessage is the public message type returned to the frontend.
// Mirrors the TypeScript ChatMessage in web/types/api.ts.
type ChatMessage struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`                // "user" | "assistant" | "reasoning" | "tool" | "error"
	Text      string `json:"text,omitempty"`      // user / assistant / reasoning / error
	Done      bool   `json:"done,omitempty"`      // assistant: always true for stored events
	ToolID    string `json:"tool_id,omitempty"`   // tool / tool_result
	ToolName  string `json:"name,omitempty"`      // tool
	Arguments string `json:"arguments,omitempty"` // tool: JSON string
	Result    any    `json:"result,omitempty"`    // tool: filled when tool result arrives
}

// SessionHistory is the HTTP response for GET /sessions/{sessionID}.
type SessionHistory struct {
	SessionID string        `json:"session_id"`
	Messages  []ChatMessage `json:"messages"`
}

func sessionHistoryHandler(svc session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sessionID := req.PathValue("sessionID")
		userID := req.URL.Query().Get("user_id")
		if sessionID == "" || userID == "" {
			http.Error(w, `{"error":"sessionID and user_id are required"}`, http.StatusBadRequest)
			return
		}

		sess, err := svc.GetSession(req.Context(), session.Key{
			AppName:   "demo-app",
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			writeJSON(w, SessionHistory{SessionID: sessionID, Messages: []ChatMessage{}})
			return
		}
		if sess == nil {
			writeJSON(w, SessionHistory{SessionID: sessionID, Messages: []ChatMessage{}})
			return
		}

		writeJSON(w, SessionHistory{
			SessionID: sessionID,
			Messages:  sessionEventsToMessages(sess),
		})
	}
}

func sessionEventsToMessages(sess *session.Session) []ChatMessage {
	msgs := make([]ChatMessage, 0, len(sess.Events))

	for _, evt := range sess.Events {
		if evt.Response == nil || evt.IsPartial || evt.IsRunnerCompletion() {
			continue
		}

		// Error
		if evt.Response.Error != nil {
			msgs = append(msgs, ChatMessage{
				ID:   evt.ID,
				Kind: "error",
				Text: evt.Response.Error.Message,
			})
			continue
		}

		for _, choice := range evt.Response.Choices {
			// Tool calls
			for _, tc := range choice.Message.ToolCalls {
				msgs = append(msgs, ChatMessage{
					ID:        evt.ID,
					Kind:      "tool",
					ToolID:    tc.ID,
					ToolName:  tc.Function.Name,
					Arguments: string(tc.Function.Arguments),
				})
			}

			// Tool result — match by ToolID to the last matching tool call
			if choice.Message.Role == model.RoleTool && choice.Message.ToolID != "" {
				var result any
				if err := json.Unmarshal([]byte(choice.Message.Content), &result); err != nil {
					result = choice.Message.Content
				}
				// Exact match by ToolID first
				matched := false
				for i := len(msgs) - 1; i >= 0; i-- {
					if msgs[i].Kind == "tool" && msgs[i].ToolID == choice.Message.ToolID {
						msgs[i].Result = result
						matched = true
						break
					}
				}
				// Fallback: match by name for the last unfinished tool
				if !matched {
					for i := len(msgs) - 1; i >= 0; i-- {
						if msgs[i].Kind == "tool" && msgs[i].ToolName == choice.Message.ToolName && msgs[i].Result == nil {
							msgs[i].Result = result
							break
						}
					}
				}
			}

			// User message
			if choice.Message.Role == model.RoleUser && choice.Message.Content != "" {
				msgs = append(msgs, ChatMessage{
					ID:   evt.ID,
					Kind: "user",
					Text: choice.Message.Content,
				})
			}

			// Assistant text is displayed before the persisted reasoning event.
			// This matches the order users see during real-time conversation.
			if choice.Message.Role == model.RoleAssistant && choice.Message.Content != "" {
				msgs = append(msgs, ChatMessage{
					ID:   evt.ID,
					Kind: "assistant",
					Text: choice.Message.Content,
					Done: true, // stored events are always complete
				})
			}

			// Reasoning
			reasoning := choice.Delta.ReasoningContent
			if reasoning == "" {
				reasoning = choice.Message.ReasoningContent
			}
			if reasoning != "" {
				msgs = append(msgs, ChatMessage{
					ID:   evt.ID,
					Kind: "reasoning",
					Text: reasoning,
				})
			}
		}
	}

	return msgs
}
