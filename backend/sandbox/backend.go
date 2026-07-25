// Package sandbox provides a pluggable execution environment for Agent tools.
// Same tool interface, interchangeable backends: LocalBackend (dev) or E2BBackend (production).
package sandbox

import (
	"context"
	"fmt"
	"path"
	"strings"
)

// SystemSkillsPath 是 Agent 在工作目录中读取系统 Skills 的固定相对路径。
const SystemSkillsPath = "skills/system"

// cleanRelativePath 把 Agent 传入的路径限制在工作目录内。
// Local 与 E2B 都只接受这一层的相对路径，不向 Agent 暴露真实文件系统路径。
func cleanRelativePath(raw string) (string, error) {
	p := strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/")
	if p == "" || p == "." {
		return ".", nil
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("absolute paths are not allowed: %s", raw)
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", fmt.Errorf("'..' is not allowed: %s", raw)
		}
	}
	return path.Clean(p), nil
}

func isSystemSkillPath(relativePath string) bool {
	return relativePath == SystemSkillsPath || strings.HasPrefix(relativePath, SystemSkillsPath+"/")
}

// FileEntry describes a filesystem entry.
type FileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" | "directory"
	Size int64  `json:"size"`
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
	// DownloadFile 是把工作区文件交给用户下载；不受 Agent 文本读取大小限制。
	DownloadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	ListDir(ctx context.Context, path string, depth int) ([]FileEntry, error)
	MakeDir(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	Move(ctx context.Context, from, to string) error
	RunCommand(ctx context.Context, cmd string, args []string, envs map[string]string) (*ExecResult, error)
	Close() error
}
