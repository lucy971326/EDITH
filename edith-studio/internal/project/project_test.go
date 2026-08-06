package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestListChildren(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "zeta.go"), "package zeta")
	writeTestFile(t, filepath.Join(projectRoot, "Alpha.md"), "# Alpha")
	if err := os.Mkdir(filepath.Join(projectRoot, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(projectRoot, "Gamma"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(projectRoot, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectRoot, "nested", "inside.txt"), "inside")

	module := newTestModule(t, projectRoot)
	entries, err := module.ListChildren("")
	if err != nil {
		t.Fatal(err)
	}
	want := []FileEntry{
		{Path: "beta", Name: "beta", Kind: EntryKindDirectory},
		{Path: "Gamma", Name: "Gamma", Kind: EntryKindDirectory},
		{Path: "nested", Name: "nested", Kind: EntryKindDirectory},
		{Path: "Alpha.md", Name: "Alpha.md", Kind: EntryKindFile},
		{Path: "zeta.go", Name: "zeta.go", Kind: EntryKindFile},
	}
	if len(entries) != len(want) {
		t.Fatalf("entry count = %d, want %d: %#v", len(entries), len(want), entries)
	}
	for index := range want {
		if entries[index] != want[index] {
			t.Errorf("entry %d = %#v, want %#v", index, entries[index], want[index])
		}
	}

	nestedEntries, err := module.ListChildren("nested")
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedEntries) != 1 || nestedEntries[0].Path != filepath.Join("nested", "inside.txt") {
		t.Fatalf("nested entries = %#v", nestedEntries)
	}
}

func TestReadText(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "nested", "main.go"), "package main\n")
	module := newTestModule(t, projectRoot)

	content, err := module.ReadText(filepath.Join("nested", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if content.Path != filepath.Join("nested", "main.go") || content.Language != "go" || content.Content != "package main\n" || content.Truncated {
		t.Fatalf("content = %#v", content)
	}
}

func TestReadTextRejectsInvalidTargets(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "file.txt"), "text")
	if err := os.Mkdir(filepath.Join(projectRoot, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	module := newTestModule(t, projectRoot)

	tests := []struct {
		name string
		path string
		want error
	}{
		{name: "absolute path", path: filepath.Join(projectRoot, "file.txt"), want: ErrInvalidPath},
		{name: "parent escape", path: filepath.Join("..", "file.txt"), want: ErrPathOutsideRoot},
		{name: "directory", path: "directory", want: ErrNotRegularFile},
		{name: "missing", path: "missing.txt", want: os.ErrNotExist},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := module.ReadText(test.path)
			if !errors.Is(err, test.want) {
				t.Fatalf("ReadText(%q) error = %v, want errors.Is(..., %v)", test.path, err, test.want)
			}
		})
	}
}

func TestListChildrenRejectsParentEscape(t *testing.T) {
	module := newTestModule(t, t.TempDir())
	_, err := module.ListChildren(filepath.Join("..", "outside"))
	if !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("ListChildren parent escape error = %v", err)
	}
}

func newTestModule(t *testing.T, projectRoot string) *Module {
	t.Helper()
	module, err := New(Dependencies{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
