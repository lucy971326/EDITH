package webapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"edith/backend-v1/internal/userconfig"
)

func TestMCPServerResponseNeverReturnsHeaderValue(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := userconfig.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	secret := "Bearer must-not-leak"
	if _, err := store.CreateMCPServer(t.Context(), "alice", userconfig.MCPServerInput{
		Name:      "GitHub",
		URL:       "https://mcp.example.com",
		Transport: "streamable",
		Enabled:   true,
		Headers:   []userconfig.MCPHeaderInput{{Name: "Authorization", Value: &secret}},
	}); err != nil {
		t.Fatal(err)
	}

	server := Server{Users: store}
	mux := http.NewServeMux()
	server.Register(mux)
	request := httptest.NewRequest(http.MethodGet, "/internal/mcp-servers?userId=alice", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if strings.Contains(body, secret) || !strings.Contains(body, `"hasValue":true`) {
		t.Fatalf("unsafe MCP response: %s", body)
	}
}
