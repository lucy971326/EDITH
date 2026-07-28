package userconfig

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestMCPServerCRUDKeepsHeaderValuesServerSide(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	secret := "Bearer very-secret"
	created, err := store.CreateMCPServer(context.Background(), "alice", MCPServerInput{
		Name:      "GitHub",
		URL:       "https://mcp.example.com",
		Transport: "streamable",
		Enabled:   true,
		Headers:   []MCPHeaderInput{{Name: "Authorization", Value: &secret}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || len(created.Headers) != 1 || created.Headers[0].Value != secret {
		t.Fatalf("created = %#v", created)
	}

	updated, err := store.UpdateMCPServer(context.Background(), "alice", created.ID, MCPServerInput{
		Name:      "GitHub",
		URL:       "https://mcp.example.com/v2",
		Transport: "streamable",
		Enabled:   false,
		Headers:   []MCPHeaderInput{{Name: "Authorization"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.URL != "https://mcp.example.com/v2" || updated.Headers[0].Value != secret {
		t.Fatalf("updated = %#v", updated)
	}

	_, err = store.UpdateMCPServer(context.Background(), "bob", created.ID, MCPServerInput{Name: "Nope", URL: "https://mcp.example.com", Transport: "sse"})
	if !errors.Is(err, ErrMCPServerNotFound) {
		t.Fatalf("cross-user update error = %v, want not found", err)
	}
	if err := store.DeleteMCPServer(context.Background(), "alice", created.ID); err != nil {
		t.Fatal(err)
	}
	servers, err := store.ListMCPServers(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("servers after delete = %#v", servers)
	}
}

func TestMCPServerRejectsLocalURL(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	_, err = store.CreateMCPServer(context.Background(), "alice", MCPServerInput{
		Name:      "Local",
		URL:       "http://127.0.0.1:3000/mcp",
		Transport: "streamable",
	})
	if !errors.Is(err, ErrInvalidMCPServer) {
		t.Fatalf("error = %v, want invalid MCP server", err)
	}
}

func TestLoadEnabledMCPServersSkipsDisabledServers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	for _, input := range []MCPServerInput{
		{Name: "Enabled", URL: "https://enabled.example.com/mcp", Transport: "streamable", Enabled: true},
		{Name: "Disabled", URL: "https://disabled.example.com/mcp", Transport: "sse", Enabled: false},
	} {
		if _, err := store.CreateMCPServer(context.Background(), "alice", input); err != nil {
			t.Fatal(err)
		}
	}

	servers, err := store.LoadEnabledMCPServers(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "Enabled" {
		t.Fatalf("enabled servers = %#v", servers)
	}
}
