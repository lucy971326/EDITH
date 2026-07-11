// Package backend 定义统一的操作接口（Backend），
// LocalBackend 对接本地磁盘，CloudSandboxBackend 对接 E2B 沙箱。
package backend

import (
	"context"
	"os"
	"time"
)

// Backend 是 Agent 操作文件系统和执行命令的抽象层。
// Agent 工具只依赖这个接口，不关心背后是本地还是云。
type Backend interface {
	// ── 读 ──
	ReadFile(ctx context.Context, path string) (string, error)

	// ── 写 ──
	WriteFile(ctx context.Context, path, content string) error

	// ── 目录 ──
	ListDir(ctx context.Context, path string) ([]DirEntry, error)
	MakeDir(ctx context.Context, path string) error

	// ── 文件信息 ──
	Exists(ctx context.Context, path string) (bool, error)
	IsDir(ctx context.Context, path string) (bool, error)
	Stat(ctx context.Context, path string) (*FileInfo, error)

	// ── 文件操作 ──
	Remove(ctx context.Context, path string) error
	Move(ctx context.Context, from, to string) error
	SearchFile(ctx context.Context, path, pattern string) ([]string, error)
	SearchContent(ctx context.Context, pattern, glob string) ([]Match, error)
	ReplaceContent(ctx context.Context, path, old, new string) error

	// ── 命令执行 ──
	ExecCommand(ctx context.Context, command, workDir string) (string, error)
}

// DirEntry 是 ListDir 返回的单个条目
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// FileInfo 是 Stat 返回的文件元信息
type FileInfo struct {
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	Size         int64       `json:"size"`
	IsDir        bool        `json:"is_dir"`
	Mode         os.FileMode `json:"mode"`
	ModifiedTime time.Time   `json:"modified_time"`
}

// Match 是 SearchContent 返回的单个匹配结果
type Match struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}
