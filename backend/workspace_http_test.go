package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"demo/sandbox"
)

func TestWorkspaceHandlersListAndDownloadOnlyCurrentWorkspace(t *testing.T) {
	provider, err := sandbox.NewLocalProvider(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	backend, err := provider.GetBackend(context.Background(), sandbox.WorkspaceID{UserID: "alice", SessionID: "session-1"})
	if err != nil {
		t.Fatalf("GetBackend() error = %v", err)
	}
	if err := backend.WriteFile(context.Background(), "result/final.txt", []byte("finished")); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := backend.WriteFile(context.Background(), ".profile", []byte("sandbox config")); err != nil {
		t.Fatalf("WriteFile(hidden) error = %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/workspace?user_id=alice&session_id=session-1&path=.", nil)
	listRec := httptest.NewRecorder()
	workspaceListHandler(provider).ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listing WorkspaceListing
	if err := json.NewDecoder(listRec.Body).Decode(&listing); err != nil {
		t.Fatalf("decode listing: %v", err)
	}
	if len(listing.Entries) != 2 || listing.Entries[0].Path != "result" || listing.Entries[1].Path != "skills" {
		t.Fatalf("listing entries = %#v", listing.Entries)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/workspace/download?user_id=alice&session_id=session-1&path=result/final.txt", nil)
	downloadRec := httptest.NewRecorder()
	workspaceDownloadHandler(provider).ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK || downloadRec.Body.String() != "finished" {
		t.Fatalf("download status = %d, body = %q", downloadRec.Code, downloadRec.Body.String())
	}

	traversalReq := httptest.NewRequest(http.MethodGet, "/workspace/download?user_id=alice&session_id=session-1&path=../secret.txt", nil)
	traversalRec := httptest.NewRecorder()
	workspaceDownloadHandler(provider).ServeHTTP(traversalRec, traversalReq)
	if traversalRec.Code != http.StatusBadRequest {
		t.Fatalf("traversal status = %d, want %d", traversalRec.Code, http.StatusBadRequest)
	}
}
