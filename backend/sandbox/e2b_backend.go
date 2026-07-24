package sandbox

import (
	"bytes"
	"context"
	"fmt"

	"github.com/eric642/e2b-go-sdk"
)

// E2B base template 中，/home/user 是我们暴露给用户的虚拟工作区根目录。
const e2bWorkspaceDir = "/home/user"

// ============================================================================
// E2BBackend executes file and command operations in one E2B sandbox.
// ============================================================================

// NewE2BBackend creates a backend for one concrete E2B sandbox.
func NewE2BBackend(sandbox *e2b.Sandbox) *E2BBackend {
	return &E2BBackend{sandbox: sandbox}
}

type E2BBackend struct {
	sandbox *e2b.Sandbox
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

func (b *E2BBackend) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return b.sandbox.Files.Read(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	return b.sandbox.Files.Read(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) WriteFile(ctx context.Context, path string, data []byte) error {
	_, err := b.sandbox.Files.Write(ctx, path, bytes.NewReader(data), e2b.FsOptions{})
	return err
}

func (b *E2BBackend) ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error) {
	entries, err := b.sandbox.Files.List(ctx, path, e2b.FsOptions{Depth: depth})
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		entryType := "file"
		if e.Type == e2b.EntryTypeDirectory {
			entryType = "directory"
		}
		out = append(out, FileEntry{
			Name: e.Name,
			Path: e.Path,
			Type: entryType,
			Size: e.Size,
		})
	}
	return out, nil
}

func (b *E2BBackend) MakeDir(ctx context.Context, path string) error {
	return b.sandbox.Files.MakeDir(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Remove(ctx context.Context, path string) error {
	return b.sandbox.Files.Remove(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Exists(ctx context.Context, path string) (bool, error) {
	return b.sandbox.Files.Exists(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Move(ctx context.Context, from, to string) error {
	return b.sandbox.Files.Move(ctx, from, to, e2b.FsOptions{})
}

// ---------------------------------------------------------------------------
// Code execution
// ---------------------------------------------------------------------------

func (b *E2BBackend) RunCommand(ctx context.Context, cmd string, args []string, envs map[string]string) (*ExecResult, error) {
	handle, err := b.sandbox.Commands.Run(ctx, cmd, e2b.RunOptions{
		Args: args,
		Envs: envs,
		Cwd:  e2bWorkspaceDir,
	})
	if err != nil {
		return nil, fmt.Errorf("run command '%s': %w", cmd, err)
	}
	result, err := handle.Wait(ctx)
	if err != nil {
		if exitErr, ok := err.(*e2b.CommandExitError); ok {
			return &ExecResult{
				Stdout:   exitErr.Result.Stdout,
				Stderr:   exitErr.Result.Stderr,
				ExitCode: int(exitErr.Result.ExitCode),
			}, nil
		}
		return nil, err
	}
	return &ExecResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: int(result.ExitCode),
	}, nil
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Close is a no-op — E2B sandboxes are persistent (auto-pause + auto-resume).
func (b *E2BBackend) Close() error { return nil }
