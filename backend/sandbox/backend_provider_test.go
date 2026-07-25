package sandbox

import (
	"context"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func TestLocalProviderReusesAndIsolatesWorkspaces(t *testing.T) {
	t.Parallel()

	provider, err := NewLocalProvider(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	ctx := context.Background()
	firstID := WorkspaceID{UserID: "alice", SessionID: "session-1"}
	secondID := WorkspaceID{UserID: "alice", SessionID: "session-2"}
	thirdID := WorkspaceID{UserID: "bob", SessionID: "session-1"}

	first, err := provider.GetBackend(ctx, firstID)
	if err != nil {
		t.Fatalf("GetBackend(first) error = %v", err)
	}
	reused, err := provider.GetBackend(ctx, firstID)
	if err != nil {
		t.Fatalf("GetBackend(reused) error = %v", err)
	}
	if first != reused {
		t.Fatal("same workspace key returned a different backend")
	}

	second, err := provider.GetBackend(ctx, secondID)
	if err != nil {
		t.Fatalf("GetBackend(second) error = %v", err)
	}
	third, err := provider.GetBackend(ctx, thirdID)
	if err != nil {
		t.Fatalf("GetBackend(third) error = %v", err)
	}
	if first == second || first == third || second == third {
		t.Fatal("different workspace keys shared a backend")
	}

	if err := first.WriteFile(ctx, "only-first.txt", []byte("first")); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for name, backend := range map[string]ExecBackend{"second": second, "third": third} {
		exists, err := backend.Exists(ctx, "only-first.txt")
		if err != nil {
			t.Fatalf("%s Exists() error = %v", name, err)
		}
		if exists {
			t.Fatalf("%s workspace can see a file from the first workspace", name)
		}
	}
}

func TestToolSetResolvesWorkspaceFromInvocation(t *testing.T) {
	t.Parallel()

	provider, err := NewLocalProvider(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	invocation := &agent.Invocation{
		Session: &session.Session{
			UserID: "alice",
			ID:     "session-42",
		},
	}
	ctx := agent.NewInvocationContext(context.Background(), invocation)
	toolSet := &sandboxToolSet{provider: provider}

	got, err := toolSet.getBackend(ctx)
	if err != nil {
		t.Fatalf("getBackend() error = %v", err)
	}
	want, err := provider.GetBackend(ctx, WorkspaceID{UserID: "alice", SessionID: "session-42"})
	if err != nil {
		t.Fatalf("GetBackend() error = %v", err)
	}
	if got != want {
		t.Fatal("toolset did not resolve the invocation workspace")
	}
}

func TestToolSetRejectsMissingInvocation(t *testing.T) {
	t.Parallel()

	provider, err := NewLocalProvider(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	toolSet := &sandboxToolSet{provider: provider}
	if _, err := toolSet.getBackend(context.Background()); err == nil {
		t.Fatal("getBackend() error = nil, want missing invocation error")
	}
}
