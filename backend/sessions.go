package main

import (
	"net/http"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/session"
)

// SessionInfo is the public summary returned by GET /sessions.
type SessionInfo struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func sessionsHandler(svc session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		userID := req.URL.Query().Get("user_id")
		if strings.TrimSpace(userID) == "" {
			http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
			return
		}

		sessions, err := svc.ListSessions(req.Context(), session.UserKey{
			AppName: "demo-app",
			UserID:  userID,
		})
		if err != nil {
			writeJSON(w, []SessionInfo{})
			return
		}

		list := make([]SessionInfo, 0, len(sessions))
		for _, s := range sessions {
			list = append(list, SessionInfo{
				ID:        s.ID,
				CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
		writeJSON(w, list)
	}
}
