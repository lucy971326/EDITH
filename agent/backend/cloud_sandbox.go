package backend

import (
	"context"
	"fmt"
	"strings"
	"time"

	e2b "github.com/eric642/e2b-go-sdk"
)

// CloudSandboxBackend 基于 E2B 云沙箱实现 Backend 接口。
type CloudSandboxBackend struct {
	client  *e2b.Client
	sandbox *e2b.Sandbox
}

// NewCloudSandboxBackend 创建 E2B Client，apiKey 读 E2B_API_KEY 环境变量时传空即可。
func NewCloudSandboxBackend(apiKey string) (*CloudSandboxBackend, error) {
	cfg := e2b.Config{}
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	client, err := e2b.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("e2b client: %w", err)
	}
	return &CloudSandboxBackend{client: client}, nil
}

// CreateSandbox 创建一个沙箱实例，template 如 "base"，timeout 为自动销毁时间。
func (b *CloudSandboxBackend) CreateSandbox(ctx context.Context, template string, timeout time.Duration) error {
	return b.CreateSandboxWithEnv(ctx, template, timeout, nil)
}

// CreateSandboxWithEnv 创建沙箱并注入环境变量。
func (b *CloudSandboxBackend) CreateSandboxWithEnv(ctx context.Context, template string, timeout time.Duration, envs map[string]string) error {
	sbx, err := b.client.Create(ctx, e2b.CreateOptions{
		Template: template,
		Timeout:  timeout,
		Envs:     envs,
	})
	if err != nil {
		return fmt.Errorf("create sandbox: %w", err)
	}
	b.sandbox = sbx
	return nil
}

// KillSandbox 销毁当前沙箱。
func (b *CloudSandboxBackend) KillSandbox(ctx context.Context) error {
	if b.sandbox == nil {
		return nil
	}
	return b.sandbox.Kill(ctx)
}

// ─── Backend 接口实现 ──────────────────────────────────────────

func (b *CloudSandboxBackend) ReadFile(ctx context.Context, path string) (string, error) {
	data, err := b.sandbox.Files.Read(ctx, path, e2b.FsOptions{})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (b *CloudSandboxBackend) WriteFile(ctx context.Context, path, content string) error {
	_, err := b.sandbox.Files.WriteString(ctx, path, content, e2b.FsOptions{})
	return err
}

func (b *CloudSandboxBackend) ListDir(ctx context.Context, path string) ([]DirEntry, error) {
	entries, err := b.sandbox.Files.List(ctx, path, e2b.FsOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, DirEntry{
			Name:  e.Name,
			IsDir: e.Type == e2b.EntryTypeDirectory,
		})
	}
	return result, nil
}

func (b *CloudSandboxBackend) MakeDir(ctx context.Context, path string) error {
	return b.sandbox.Files.MakeDir(ctx, path, e2b.FsOptions{})
}

func (b *CloudSandboxBackend) Exists(ctx context.Context, path string) (bool, error) {
	return b.sandbox.Files.Exists(ctx, path, e2b.FsOptions{})
}

func (b *CloudSandboxBackend) IsDir(ctx context.Context, path string) (bool, error) {
	return b.sandbox.Files.IsDir(ctx, path, e2b.FsOptions{})
}

func (b *CloudSandboxBackend) Stat(ctx context.Context, path string) (*FileInfo, error) {
	st, err := b.sandbox.Files.Stat(ctx, path, e2b.FsOptions{})
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		Name:         st.Name,
		Path:         st.Path,
		Size:         st.Size,
		IsDir:        st.Type == e2b.EntryTypeDirectory,
		ModifiedTime: st.ModifiedTime,
	}, nil
}

func (b *CloudSandboxBackend) Remove(ctx context.Context, path string) error {
	return b.sandbox.Files.Remove(ctx, path, e2b.FsOptions{})
}

func (b *CloudSandboxBackend) Move(ctx context.Context, from, to string) error {
	return b.sandbox.Files.Move(ctx, from, to, e2b.FsOptions{})
}

func (b *CloudSandboxBackend) SearchFile(ctx context.Context, path, pattern string) ([]string, error) {
	cmd, err := b.sandbox.Commands.Run(ctx, "sh", e2b.RunOptions{
		Args: []string{"-c", fmt.Sprintf("find %s -name '%s' -type f 2>/dev/null", path, pattern)},
	})
	if err != nil {
		return nil, err
	}
	res, err := cmd.Wait(ctx)
	if err != nil {
		return nil, err
	}
	var results []string
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results, nil
}

func (b *CloudSandboxBackend) SearchContent(ctx context.Context, pattern, glob string) ([]Match, error) {
	grepPattern := pattern
	if glob != "" {
		grepPattern = fmt.Sprintf("%s --include='%s'", pattern, glob)
	}
	cmd, err := b.sandbox.Commands.Run(ctx, "sh", e2b.RunOptions{
		Args: []string{"-c", fmt.Sprintf("grep -rn %s . 2>/dev/null", grepPattern)},
	})
	if err != nil {
		return nil, err
	}
	res, err := cmd.Wait(ctx)
	if err != nil && res == nil {
		return nil, err
	}
	var matches []Match
	for _, line := range strings.Split(res.Stdout, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum := 0
		fmt.Sscanf(parts[1], "%d", &lineNum)
		matches = append(matches, Match{
			Path:    parts[0],
			Line:    lineNum,
			Content: strings.TrimSpace(parts[2]),
		})
	}
	return matches, nil
}

func (b *CloudSandboxBackend) ReplaceContent(ctx context.Context, path, old, new string) error {
	cmd, err := b.sandbox.Commands.Run(ctx, "sh", e2b.RunOptions{
		Args: []string{"-c", fmt.Sprintf("sed -i 's/%s/%s/g' %s", old, new, path)},
	})
	if err != nil {
		return err
	}
	_, err = cmd.Wait(ctx)
	return err
}

func (b *CloudSandboxBackend) ExecCommand(ctx context.Context, command, workDir string) (string, error) {
	cmd, err := b.sandbox.Commands.Run(ctx, "sh", e2b.RunOptions{
		Args: []string{"-c", command},
		Cwd:  workDir,
	})
	if err != nil {
		return "", err
	}
	res, err := cmd.Wait(ctx)
	if err != nil {
		return res.Stderr, fmt.Errorf("command failed: %w\n%s", err, res.Stderr)
	}
	return res.Stdout, nil
}
