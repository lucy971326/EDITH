package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProviderMountsSystemSkillsReadOnly(t *testing.T) {
	systemRoot := t.TempDir()
	skillFile := filepath.Join(systemRoot, "learn", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillFile, []byte("skill content"), 0644); err != nil {
		t.Fatal(err)
	}

	provider, err := NewLocalProvider(t.TempDir(), systemRoot)
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	backend, err := provider.GetBackend(context.Background(), WorkspaceID{UserID: "alice", SessionID: "s1"})
	if err != nil {
		t.Fatalf("GetBackend() error = %v", err)
	}
	data, err := backend.ReadFile(context.Background(), "skills/system/learn/SKILL.md")
	if err != nil || string(data) != "skill content" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
	if err := backend.WriteFile(context.Background(), "skills/system/learn/SKILL.md", []byte("changed")); err == nil {
		t.Fatal("WriteFile() error = nil, want read-only system skills error")
	}
}

func TestLocalBackendListsVirtualSkillsDirectory(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	backend, err := provider.GetBackend(context.Background(), WorkspaceID{UserID: "alice", SessionID: "s1"})
	if err != nil {
		t.Fatalf("GetBackend() error = %v", err)
	}
	entries, err := backend.ListDir(context.Background(), ".", 1)
	if err != nil || len(entries) != 1 || entries[0].Path != "skills" {
		t.Fatalf("ListDir(root) = %#v, %v", entries, err)
	}
	entries, err = backend.ListDir(context.Background(), "skills", 1)
	if err != nil || len(entries) != 2 || entries[0].Path != "skills/system" || entries[1].Path != "skills/user" {
		t.Fatalf("ListDir(skills) = %#v, %v", entries, err)
	}
}

func TestCleanRelativePathRejectsPhysicalPaths(t *testing.T) {
	for _, input := range []string{"/home/user/secret", "../secret", "a/../secret"} {
		if _, err := cleanRelativePath(input); err == nil {
			t.Errorf("cleanRelativePath(%q) error = nil", input)
		}
	}
	if got, err := cleanRelativePath("skills/system/demo/SKILL.md"); err != nil || got != "skills/system/demo/SKILL.md" {
		t.Errorf("cleanRelativePath() = %q, %v", got, err)
	}
}

func TestRelativeE2BPathDoesNotExposeWorkspaceRoot(t *testing.T) {
	got, err := relativeE2BPath("/home/user/output/report.md")
	if err != nil || got != "output/report.md" {
		t.Fatalf("relativeE2BPath() = %q, %v", got, err)
	}
	if _, err := relativeE2BPath("/etc/passwd"); err == nil {
		t.Fatal("relativeE2BPath() error = nil for path outside workspace")
	}
}
