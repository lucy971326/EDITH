package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"edith/studio/internal/project"
	"edith/studio/internal/workspace"
)

func TestFileHandlers(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectModule, err := project.New(project.Dependencies{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(context.Background(), &workspace.Workspace{Project: projectModule})

	t.Run("lists project root", func(t *testing.T) {
		response := serveRequest(handler, http.MethodGet, "/api/files", "")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var body struct {
			Entries []project.FileEntry `json:"entries"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Entries) != 1 || body.Entries[0].Path != "main.go" {
			t.Fatalf("entries = %#v", body.Entries)
		}
	})

	t.Run("reads text file", func(t *testing.T) {
		response := serveRequest(handler, http.MethodGet, "/api/files/content", "main.go")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var body project.FileContent
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Content != "package main\n" || body.Language != "go" {
			t.Fatalf("content = %#v", body)
		}
	})

	t.Run("rejects parent escape", func(t *testing.T) {
		response := serveRequest(handler, http.MethodGet, "/api/files/content", "../outside.txt")
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})

	t.Run("reports missing file", func(t *testing.T) {
		response := serveRequest(handler, http.MethodGet, "/api/files/content", "missing.go")
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})
}

func serveRequest(handler http.Handler, method, endpoint, path string) *httptest.ResponseRecorder {
	requestURL := endpoint
	if path != "" {
		requestURL += "?" + url.Values{"path": []string{path}}.Encode()
	}
	request := httptest.NewRequest(method, requestURL, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
