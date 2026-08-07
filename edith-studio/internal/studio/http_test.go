package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"edith/studio/internal/mcp"
	"edith/studio/internal/models"
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

func TestModelHandlerReturnsBackendCatalog(t *testing.T) {
	modelModule, err := models.Build(models.Config{
		Default: "deepseek-pro",
		Providers: map[string]models.ProviderConfig{
			"deepseek": {APIKey: "secret", BaseURL: "https://api.deepseek.com", Variant: "deepseek"},
		},
		Models: map[string]models.ModelConfig{
			"deepseek-pro": {
				Provider:      "deepseek",
				Name:          "deepseek-v4-pro",
				ContextWindow: 1_000_000,
				Thinking:      models.ThinkingConfig{Default: "max", Modes: []string{"off", "high", "max"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build models: %v", err)
	}

	handler := newHandler(context.Background(), &workspace.Workspace{Models: modelModule})
	response := serveRequest(handler, http.MethodGet, "/api/models", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var catalog models.Catalog
	if err := json.NewDecoder(response.Body).Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.DefaultModelID != "deepseek-pro" || len(catalog.Models) != 1 || catalog.Models[0].ContextWindow != 1_000_000 {
		t.Fatalf("catalog = %#v", catalog)
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("response leaked API key: %s", response.Body.String())
	}
}

func TestMCPHandlerReturnsServerStatuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// 用户级配置一个连不上的 server：验证失败不致命且状态透传到 HTTP。
	edithDir := filepath.Join(home, ".edith")
	if err := os.MkdirAll(edithDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(edithDir, "mcp.json"), []byte(`{
	  "servers": {"broken": {"transport": "stdio", "command": "no-such-edith-command", "timeout": "1s"}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mcpModule, err := mcp.New(mcp.Dependencies{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("new mcp: %v", err)
	}

	handler := newHandler(context.Background(), &workspace.Workspace{MCP: mcpModule})
	response := serveRequest(handler, http.MethodGet, "/api/mcp", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Servers []mcp.ServerStatus `json:"servers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Servers) != 1 || body.Servers[0].Name != "broken" || body.Servers[0].Status != "error" {
		t.Fatalf("servers = %#v", body.Servers)
	}
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
