package backend

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// LocalBackend 基于本地文件系统 + shell 实现 Backend 接口。
// 它是开发阶段的默认后端，生产环境可替换为云沙箱实现。
type LocalBackend struct {
	baseDir string
}

// NewLocalBackend 创建 LocalBackend，baseDir 是 Agent 能操作的最大范围。
func NewLocalBackend(baseDir string) *LocalBackend {
	return &LocalBackend{baseDir: baseDir}
}

// resolve 把相对路径转为绝对路径，并防止路径逃逸
func (b *LocalBackend) resolve(path string) (string, error) {
	p := filepath.Join(b.baseDir, path)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	base, _ := filepath.Abs(b.baseDir)
	if !strings.HasPrefix(abs, base) {
		return "", fmt.Errorf("path escapes base directory: %s", path)
	}
	return abs, nil
}

func (b *LocalBackend) ReadFile(_ context.Context, path string) (string, error) {
	p, err := b.resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (b *LocalBackend) WriteFile(_ context.Context, path, content string) error {
	p, err := b.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0644)
}

func (b *LocalBackend) ListDir(_ context.Context, path string) ([]DirEntry, error) {
	p, err := b.resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return result, nil
}

func (b *LocalBackend) SearchContent(_ context.Context, pattern, glob string) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	var matches []Match
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != "" {
			if ok, _ := filepath.Match(glob, d.Name()); !ok {
				return nil
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(b.baseDir, path)
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				matches = append(matches, Match{
					Path:    filepath.ToSlash(relPath),
					Line:    i + 1,
					Content: strings.TrimSpace(line),
				})
			}
		}
		return nil
	}

	return matches, filepath.WalkDir(b.baseDir, walk)
}

func (b *LocalBackend) ExecCommand(ctx context.Context, command, workDir string) (string, error) {
	p, err := b.resolve(workDir)
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}
	cmd.Dir = p
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), fmt.Errorf("command failed: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (b *LocalBackend) MakeDir(_ context.Context, path string) error {
	p, err := b.resolve(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0755)
}

func (b *LocalBackend) Exists(_ context.Context, path string) (bool, error) {
	p, err := b.resolve(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (b *LocalBackend) IsDir(_ context.Context, path string) (bool, error) {
	p, err := b.resolve(path)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func (b *LocalBackend) Stat(_ context.Context, path string) (*FileInfo, error) {
	p, err := b.resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		Name:         info.Name(),
		Path:         path,
		Size:         info.Size(),
		IsDir:        info.IsDir(),
		Mode:         info.Mode(),
		ModifiedTime: info.ModTime(),
	}, nil
}

func (b *LocalBackend) Remove(_ context.Context, path string) error {
	p, err := b.resolve(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func (b *LocalBackend) Move(_ context.Context, from, to string) error {
	src, err := b.resolve(from)
	if err != nil {
		return err
	}
	dst, err := b.resolve(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func (b *LocalBackend) SearchFile(_ context.Context, path, pattern string) ([]string, error) {
	p, err := b.resolve(path)
	if err != nil {
		return nil, err
	}
	var results []string
	err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ok, _ := doublestar.Match(pattern, d.Name()); ok {
			rel, _ := filepath.Rel(b.baseDir, path)
			results = append(results, filepath.ToSlash(rel))
		}
		return nil
	})
	return results, err
}

func (b *LocalBackend) ReplaceContent(_ context.Context, path, old, new string) error {
	p, err := b.resolve(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(data), old, new)
	return os.WriteFile(p, []byte(updated), 0644)
}
