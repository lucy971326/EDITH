package workspace

import (
	"path/filepath"
	"testing"
)

func TestWorkspaceIDIsStableAndProjectScoped(t *testing.T) {
	firstProjectRoot := filepath.Join(t.TempDir(), "first")
	secondProjectRoot := filepath.Join(t.TempDir(), "second")
	if workspaceID(firstProjectRoot) != workspaceID(firstProjectRoot) {
		t.Fatal("workspace ID changed for the same project root")
	}
	if workspaceID(firstProjectRoot) == workspaceID(secondProjectRoot) {
		t.Fatal("workspace ID matched for different project roots")
	}
}
