package httpserver

import (
	"encoding/json"
	"net/http"

	"github-agent/edith/internal/identity"
)

func meHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := identity.ClerkUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{"clerk_user_id": userID})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}