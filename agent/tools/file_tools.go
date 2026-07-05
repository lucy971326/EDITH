// Package tools 把 Backend 接口方法包装成 FunctionTool，注册到 Agent。
// Agent 通过工具集调用文件操作和命令执行，不关心背后是本地还是云。
package tools

import (
	"context"
	"fmt"

	"github-agent/agent/backend"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// FileToolSet 实现 tool.ToolSet 接口，向 Agent 提供 5 个文件/命令工具。
type FileToolSet struct {
	name    string
	backend backend.Backend
	tools   []tool.Tool
}

// FileToolOption 是 FileToolSet 的配置函数
type FileToolOption func(*FileToolSet)

// WithToolSetName 自定义工具集名称（默认 "file"）
func WithToolSetName(name string) FileToolOption {
	return func(s *FileToolSet) { s.name = name }
}

// NewFileToolSet 创建文件工具集，所有操作最终由 backend 执行。
func NewFileToolSet(b backend.Backend, opts ...FileToolOption) *FileToolSet {
	s := &FileToolSet{name: "file", backend: b}
	for _, opt := range opts {
		opt(s)
	}
	s.tools = []tool.Tool{
		s.makeReadFile(),
		s.makeWriteFile(),
		s.makeListDir(),
		s.makeSearchContent(),
		s.makeExecCommand(),
	}
	return s
}

// ── tool.ToolSet 接口 ──

func (s *FileToolSet) Tools(_ context.Context) []tool.Tool { return s.tools }
func (s *FileToolSet) Close() error                        { return nil }
func (s *FileToolSet) Name() string                        { return s.name }

// ── 工具：read_file ──

type readFileArgs struct {
	Path string `json:"path" jsonschema:"description=文件路径，相对 workspace 根目录"`
}

type readFileResult struct {
	Content string `json:"content"`
}

func (s *FileToolSet) makeReadFile() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args readFileArgs) (readFileResult, error) {
			content, err := s.backend.ReadFile(ctx, args.Path)
			if err != nil {
				return readFileResult{}, fmt.Errorf("read_file: %w", err)
			}
			return readFileResult{Content: content}, nil
		},
		function.WithName("read_file"),
		function.WithDescription("读取 workspace 中的文件内容"),
	)
}

// ── 工具：write_file ──

type writeFileArgs struct {
	Path    string `json:"path" jsonschema:"description=文件路径，相对 workspace 根目录"`
	Content string `json:"content" jsonschema:"description=要写入的文本内容"`
}

type writeFileResult struct {
	Message string `json:"message"`
}

func (s *FileToolSet) makeWriteFile() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args writeFileArgs) (writeFileResult, error) {
			if err := s.backend.WriteFile(ctx, args.Path, args.Content); err != nil {
				return writeFileResult{}, fmt.Errorf("write_file: %w", err)
			}
			return writeFileResult{Message: "written"}, nil
		},
		function.WithName("write_file"),
		function.WithDescription("写入文件到 workspace，自动创建父目录"),
	)
}

// ── 工具：list_dir ──

type listDirArgs struct {
	Path string `json:"path" jsonschema:"description=目录路径，相对 workspace 根目录"`
}

type listDirResult struct {
	Entries []backend.DirEntry `json:"entries"`
}

func (s *FileToolSet) makeListDir() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args listDirArgs) (listDirResult, error) {
			entries, err := s.backend.ListDir(ctx, args.Path)
			if err != nil {
				return listDirResult{}, fmt.Errorf("list_dir: %w", err)
			}
			return listDirResult{Entries: entries}, nil
		},
		function.WithName("list_dir"),
		function.WithDescription("列出目录下的文件和文件夹"),
	)
}

// ── 工具：search_content ──

type searchContentArgs struct {
	Pattern string `json:"pattern" jsonschema:"description=正则表达式，搜索文件内容"`
	Glob    string `json:"glob,omitempty" jsonschema:"description=文件通配符过滤，如 *.go"`
}

type searchContentResult struct {
	Matches []backend.Match `json:"matches"`
}

func (s *FileToolSet) makeSearchContent() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args searchContentArgs) (searchContentResult, error) {
			matches, err := s.backend.SearchContent(ctx, args.Pattern, args.Glob)
			if err != nil {
				return searchContentResult{}, fmt.Errorf("search_content: %w", err)
			}
			return searchContentResult{Matches: matches}, nil
		},
		function.WithName("search_content"),
		function.WithDescription("用正则搜索文件内容，支持 glob 过滤文件名"),
	)
}

// ── 工具：exec_command ──

type execCommandArgs struct {
	Command string `json:"command" jsonschema:"description=要执行的 shell 命令"`
	WorkDir string `json:"work_dir" jsonschema:"description=工作目录，相对 workspace 根目录"`
}

type execCommandResult struct {
	Output string `json:"output"`
}

func (s *FileToolSet) makeExecCommand() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args execCommandArgs) (execCommandResult, error) {
			output, err := s.backend.ExecCommand(ctx, args.Command, args.WorkDir)
			if err != nil {
				return execCommandResult{}, fmt.Errorf("exec_command: %w", err)
			}
			return execCommandResult{Output: output}, nil
		},
		function.WithName("exec_command"),
		function.WithDescription("在 workspace 中执行 shell 命令"),
		function.WithLongRunning(true),
	)
}
