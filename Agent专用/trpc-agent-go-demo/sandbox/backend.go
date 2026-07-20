// Package sandbox provides a pluggable execution environment for Agent tools.
// Same tool interface, interchangeable backends: LocalBackend (dev) or E2BBackend (production).
package sandbox

import "context"

// FileEntry describes a filesystem entry.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"` // "file" | "directory"
	Size  int64  `json:"size"`
}

// ExecResult is the output of a command execution.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// ExecBackend abstracts the execution environment.
// Implementations: LocalBackend (os + os/exec), E2BBackend (e2b-go-sdk cloud sandbox).
type ExecBackend interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error)
	MakeDir(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	Move(ctx context.Context, from, to string) error
	RunCommand(ctx context.Context, cmd string, args []string, envs map[string]string) (*ExecResult, error)
	Close() error
}
