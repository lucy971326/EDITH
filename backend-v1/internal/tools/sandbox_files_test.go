package tools

import (
	"testing"

	"edith/backend-v1/internal/sandbox"
)

func TestSandboxPathKeepsToolsInsideWorkspace(t *testing.T) {
	path, err := sandboxPath("project/main.go", false)
	if err != nil {
		t.Fatalf("sandboxPath() error = %v", err)
	}
	if path != sandbox.Workspace.Root+"/project/main.go" {
		t.Fatalf("sandboxPath() = %q", path)
	}

	for _, input := range []string{"/etc/passwd", "../secret", "project/../../secret"} {
		if _, err := sandboxPath(input, false); err == nil {
			t.Fatalf("sandboxPath(%q) error = nil", input)
		}
	}
}

func TestSandboxPathAllowsRootOnlyForDirectoryListing(t *testing.T) {
	path, err := sandboxPath("", true)
	if err != nil {
		t.Fatalf("sandboxPath(root) error = %v", err)
	}
	if path != sandbox.Workspace.Root {
		t.Fatalf("sandboxPath(root) = %q", path)
	}

	if _, err := sandboxPath("", false); err == nil {
		t.Fatal("sandboxPath(root, false) error = nil")
	}
}
