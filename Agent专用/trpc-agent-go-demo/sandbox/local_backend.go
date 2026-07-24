package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// LocalBackend executes file and command operations in one local directory.
// ============================================================================

const (
	defaultCreateDirMode  = os.FileMode(0755) // rwxr-xr-x
	defaultCreateFileMode = os.FileMode(0644) // rw-r--r--
	defaultMaxFileSize    = 1024 * 1024       // 1 MB
)

// NewLocalBackend creates a LocalBackend rooted at baseDir.
func NewLocalBackend(baseDir string) (*LocalBackend, error) {
	abs, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return nil, fmt.Errorf("resolve base directory '%s': %w", baseDir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("base directory '%s' does not exist: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base directory '%s' is not a directory", abs)
	}
	return &LocalBackend{baseDir: abs}, nil
}

type LocalBackend struct {
	baseDir string
}

// resolvePath validates a path to prevent directory traversal attacks,
// and resolves a relative path within the base directory.
// Mirrors the filetool implementation.
func (b *LocalBackend) resolvePath(relativePath string) (string, error) {
	reqPath := strings.TrimSpace(relativePath)
	if filepath.IsAbs(reqPath) || strings.Contains(reqPath, "..") {
		return "", fmt.Errorf("invalid path - absolute paths and '..' are not allowed: %s", relativePath)
	}
	return filepath.Join(b.baseDir, reqPath), nil
}

func (b *LocalBackend) BaseDir() string { return b.baseDir }

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

func (b *LocalBackend) ReadFile(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot access file '%s': %w", path, err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("target path '%s' is a directory", path)
	}
	if stat.Size() > defaultMaxFileSize {
		return nil, fmt.Errorf("file is too large: %d > %d", stat.Size(), defaultMaxFileSize)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("reading file '%s': %w", path, err)
	}
	return data, nil
}

func (b *LocalBackend) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("downloading file %q: %w", path, err)
	}
	return data, nil
}

func (b *LocalBackend) WriteFile(ctx context.Context, path string, data []byte) error {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return err
	}
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, defaultCreateDirMode); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}
	if err := os.WriteFile(fullPath, data, defaultCreateFileMode); err != nil {
		return fmt.Errorf("writing to file '%s': %w", path, err)
	}
	return nil
}

func (b *LocalBackend) ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error) {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access '%s': %w", path, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("path '%s' is not a directory", path)
	}
	if depth <= 0 {
		depth = 1
	}
	return b.listRecursive(fullPath, "", depth)
}

func (b *LocalBackend) listRecursive(root string, relativePrefix string, depth int) ([]FileEntry, error) {
	if depth <= 0 {
		return nil, nil
	}
	dirPath := filepath.Join(root, relativePrefix)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var result []FileEntry
	for _, entry := range entries {
		relPath := filepath.Join(relativePrefix, entry.Name())
		info, _ := entry.Info()
		size := int64(0)
		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		} else if info != nil {
			size = info.Size()
		}
		result = append(result, FileEntry{
			Name: entry.Name(),
			Path: filepath.ToSlash(relPath),
			Type: entryType,
			Size: size,
		})
		if entry.IsDir() && depth > 1 {
			children, err := b.listRecursive(root, relPath, depth-1)
			if err != nil {
				return result, err
			}
			result = append(result, children...)
		}
	}
	return result, nil
}

func (b *LocalBackend) MakeDir(ctx context.Context, path string) error {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(fullPath, defaultCreateDirMode)
}

func (b *LocalBackend) Remove(ctx context.Context, path string) error {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullPath)
}

func (b *LocalBackend) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := b.resolvePath(path)
	if err != nil {
		return false, err
	}
	_, statErr := os.Stat(fullPath)
	if os.IsNotExist(statErr) {
		return false, nil
	}
	return statErr == nil, statErr
}

func (b *LocalBackend) Move(ctx context.Context, from, to string) error {
	fromPath, err := b.resolvePath(from)
	if err != nil {
		return err
	}
	toPath, err := b.resolvePath(to)
	if err != nil {
		return err
	}
	return os.Rename(fromPath, toPath)
}

// ---------------------------------------------------------------------------
// Code execution
// ---------------------------------------------------------------------------

func (b *LocalBackend) RunCommand(ctx context.Context, cmd string, args []string, envs map[string]string) (*ExecResult, error) {
	if deadline, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		_ = deadline // suppress unused warning
	}
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = b.baseDir

	c.Env = os.Environ()
	for k, v := range envs {
		c.Env = append(c.Env, k+"="+v)
	}

	output, err := c.CombinedOutput()
	result := &ExecResult{
		Stdout:   string(output),
		ExitCode: 0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
		} else {
			return result, fmt.Errorf("run command '%s': %w", cmd, err)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (b *LocalBackend) Close() error { return nil }
