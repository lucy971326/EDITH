// Package backend 定义统一的操作接口（Backend），
// LocalBackend 对接本地磁盘，未来可对接云沙箱。
package backend

import "context"

// Backend 是 Agent 操作文件系统和执行命令的抽象层。
// Agent 工具只依赖这个接口，不关心背后是本地还是云。
type Backend interface {
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path, content string) error
	ListDir(ctx context.Context, path string) ([]DirEntry, error)
	SearchContent(ctx context.Context, pattern, glob string) ([]Match, error)
	ExecCommand(ctx context.Context, command, workDir string) (string, error)
}

// DirEntry 是 ListDir 返回的单个条目
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// Match 是 SearchContent 返回的单个匹配结果
type Match struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}
