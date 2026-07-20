package sandbox

import (
	"bytes"
	"context"
	"fmt"

	"github.com/eric642/e2b-go-sdk"
)

// ============================================================================
// E2BBackend — execute in a cloud sandbox via e2b-go-sdk.
// ============================================================================

// NewE2BBackend creates an E2BBackend for the given user.
func NewE2BBackend(mgr *SandboxManager, userID string) *E2BBackend {
	return &E2BBackend{mgr: mgr, userID: userID}
}

type E2BBackend struct {
	mgr    *SandboxManager
	userID string
}

func (b *E2BBackend) sandbox(ctx context.Context) (*e2b.Sandbox, error) {
	return b.mgr.GetSandbox(ctx, b.userID)
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

func (b *E2BBackend) ReadFile(ctx context.Context, path string) ([]byte, error) {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return nil, err
	}
	return sbx.Files.Read(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) WriteFile(ctx context.Context, path string, data []byte) error {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return err
	}
	_, err = sbx.Files.Write(ctx, path, bytes.NewReader(data), e2b.FsOptions{})
	return err
}

func (b *E2BBackend) ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error) {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := sbx.Files.List(ctx, path, e2b.FsOptions{Depth: depth})
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
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return err
	}
	return sbx.Files.MakeDir(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Remove(ctx context.Context, path string) error {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return err
	}
	return sbx.Files.Remove(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Exists(ctx context.Context, path string) (bool, error) {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return false, err
	}
	return sbx.Files.Exists(ctx, path, e2b.FsOptions{})
}

func (b *E2BBackend) Move(ctx context.Context, from, to string) error {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return err
	}
	return sbx.Files.Move(ctx, from, to, e2b.FsOptions{})
}

// ---------------------------------------------------------------------------
// Code execution
// ---------------------------------------------------------------------------

func (b *E2BBackend) RunCommand(ctx context.Context, cmd string, args []string, envs map[string]string) (*ExecResult, error) {
	sbx, err := b.sandbox(ctx)
	if err != nil {
		return nil, err
	}
	handle, err := sbx.Commands.Run(ctx, cmd, e2b.RunOptions{
		Args: args,
		Envs: envs,
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
