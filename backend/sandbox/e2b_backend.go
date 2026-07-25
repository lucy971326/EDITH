package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/eric642/e2b-go-sdk"
)

// E2B base template 中，/home/user 是 E2B 的真实工作目录。
// Agent 只传相对路径，真实根目录只在这个 Adapter 内部使用。
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

func (b *E2BBackend) resolvePath(input string) (string, error) {
	relativePath, err := cleanRelativePath(input)
	if err != nil {
		return "", err
	}
	return path.Join(e2bWorkspaceDir, relativePath), nil
}

func relativeE2BPath(realPath string) (string, error) {
	cleaned := path.Clean(realPath)
	if cleaned == e2bWorkspaceDir {
		return ".", nil
	}
	prefix := e2bWorkspaceDir + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", fmt.Errorf("E2B returned a path outside the workspace: %s", realPath)
	}
	return strings.TrimPrefix(cleaned, prefix), nil
}

func rejectSystemSkillWrite(relativePath string) error {
	if isSystemSkillPath(relativePath) {
		return fmt.Errorf("system skills are read-only: %s", relativePath)
	}
	return nil
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

func (b *E2BBackend) ReadFile(ctx context.Context, path string) ([]byte, error) {
	resolved, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return b.sandbox.Files.Read(ctx, resolved, e2b.FsOptions{})
}

func (b *E2BBackend) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	return b.ReadFile(ctx, path)
}

func (b *E2BBackend) WriteFile(ctx context.Context, path string, data []byte) error {
	relativePath, err := cleanRelativePath(path)
	if err != nil {
		return err
	}
	if err := rejectSystemSkillWrite(relativePath); err != nil {
		return err
	}
	resolved, err := b.resolvePath(relativePath)
	if err != nil {
		return err
	}
	_, err = b.sandbox.Files.Write(ctx, resolved, bytes.NewReader(data), e2b.FsOptions{})
	return err
}

func (b *E2BBackend) ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error) {
	resolved, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	entries, err := b.sandbox.Files.List(ctx, resolved, e2b.FsOptions{Depth: depth})
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		relativePath, pathErr := relativeE2BPath(e.Path)
		if pathErr != nil {
			return nil, pathErr
		}
		entryType := "file"
		if e.Type == e2b.EntryTypeDirectory {
			entryType = "directory"
		}
		out = append(out, FileEntry{
			Name: e.Name,
			Path: relativePath,
			Type: entryType,
			Size: e.Size,
		})
	}
	return out, nil
}

func (b *E2BBackend) MakeDir(ctx context.Context, path string) error {
	relativePath, err := cleanRelativePath(path)
	if err != nil {
		return err
	}
	if err := rejectSystemSkillWrite(relativePath); err != nil {
		return err
	}
	resolved, err := b.resolvePath(relativePath)
	if err != nil {
		return err
	}
	return b.sandbox.Files.MakeDir(ctx, resolved, e2b.FsOptions{})
}

func (b *E2BBackend) Remove(ctx context.Context, path string) error {
	relativePath, err := cleanRelativePath(path)
	if err != nil {
		return err
	}
	if err := rejectSystemSkillWrite(relativePath); err != nil {
		return err
	}
	resolved, err := b.resolvePath(relativePath)
	if err != nil {
		return err
	}
	return b.sandbox.Files.Remove(ctx, resolved, e2b.FsOptions{})
}

func (b *E2BBackend) Exists(ctx context.Context, path string) (bool, error) {
	resolved, err := b.resolvePath(path)
	if err != nil {
		return false, err
	}
	return b.sandbox.Files.Exists(ctx, resolved, e2b.FsOptions{})
}

func (b *E2BBackend) Move(ctx context.Context, from, to string) error {
	relativeFrom, err := cleanRelativePath(from)
	if err != nil {
		return err
	}
	relativeTo, err := cleanRelativePath(to)
	if err != nil {
		return err
	}
	if err := rejectSystemSkillWrite(relativeFrom); err != nil {
		return err
	}
	if err := rejectSystemSkillWrite(relativeTo); err != nil {
		return err
	}
	resolvedFrom, err := b.resolvePath(relativeFrom)
	if err != nil {
		return err
	}
	resolvedTo, err := b.resolvePath(relativeTo)
	if err != nil {
		return err
	}
	return b.sandbox.Files.Move(ctx, resolvedFrom, resolvedTo, e2b.FsOptions{})
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
