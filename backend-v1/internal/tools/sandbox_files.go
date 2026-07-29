package tools

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"edith/backend-v1/internal/sandbox"
	"github.com/eric642/e2b-go-sdk"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

const defaultFileReadLimit = 32 * 1024

type sandboxPathInput struct {
	Path string `json:"path" jsonschema:"description=Sandbox workspace-relative path. Empty means the workspace root."`
}

type sandboxListFilesInput struct {
	Path  string `json:"path,omitempty" jsonschema:"description=Sandbox workspace-relative directory. Empty means the workspace root."`
	Depth int    `json:"depth,omitempty" jsonschema:"description=Directory depth to list. Default is 1."`
}

type sandboxReadFileInput struct {
	Path   string `json:"path" jsonschema:"description=Sandbox workspace-relative text file path,required"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=Zero-based byte offset. Default is 0."`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum bytes to return. Default is 32768."`
}

type sandboxWriteFileInput struct {
	Path    string `json:"path" jsonschema:"description=Sandbox workspace-relative file path,required"`
	Content string `json:"content" jsonschema:"description=Text content to create or overwrite,required"`
}

type sandboxMovePathInput struct {
	From string `json:"from" jsonschema:"description=Source workspace-relative path,required"`
	To   string `json:"to" jsonschema:"description=Destination workspace-relative path,required"`
}

type sandboxFileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type sandboxListFilesOutput struct {
	Path    string             `json:"path"`
	Entries []sandboxFileEntry `json:"entries"`
}

type sandboxReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type sandboxFileOutput struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (s *SandboxToolSet) listFilesTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxListFilesInput) (sandboxListFilesOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxListFilesOutput{}, err
			}
			directory, err := sandboxPath(input.Path, true)
			if err != nil {
				return sandboxListFilesOutput{}, err
			}
			depth := input.Depth
			if depth == 0 {
				depth = 1
			}
			entries, err := workspace.Files.List(ctx, directory, e2b.FsOptions{Depth: depth})
			if err != nil {
				return sandboxListFilesOutput{}, err
			}

			output := sandboxListFilesOutput{Path: sandboxRelativePath(directory), Entries: []sandboxFileEntry{}}
			for _, entry := range entries {
				output.Entries = append(output.Entries, sandboxFileEntry{
					Name: entry.Name,
					Path: sandboxRelativePath(entry.Path),
					Type: sandboxEntryType(entry.Type),
					Size: entry.Size,
				})
			}
			return output, nil
		},
		function.WithName("sandbox_list_files"),
		function.WithDescription(sandboxToolDescription("列出当前会话 Sandbox 工作区中的文件和目录。")),
	)
}

func (s *SandboxToolSet) readFileTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxReadFileInput) (sandboxReadFileOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxReadFileOutput{}, err
			}
			filePath, err := sandboxPath(input.Path, false)
			if err != nil {
				return sandboxReadFileOutput{}, err
			}
			if input.Offset < 0 {
				return sandboxReadFileOutput{}, fmt.Errorf("file offset cannot be negative")
			}
			limit := input.Limit
			if limit == 0 {
				limit = defaultFileReadLimit
			}
			if limit < 1 {
				return sandboxReadFileOutput{}, fmt.Errorf("file limit must be positive")
			}

			reader, err := workspace.Files.ReadStream(ctx, filePath, e2b.FsOptions{})
			if err != nil {
				return sandboxReadFileOutput{}, err
			}
			defer reader.Close()
			if input.Offset > 0 {
				if _, err := io.CopyN(io.Discard, reader, int64(input.Offset)); err != nil && err != io.EOF {
					return sandboxReadFileOutput{}, err
				}
			}
			data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
			if err != nil {
				return sandboxReadFileOutput{}, err
			}
			truncated := len(data) > limit
			if truncated {
				data = data[:limit]
			}
			return sandboxReadFileOutput{
				Path:      sandboxRelativePath(filePath),
				Content:   string(data),
				Truncated: truncated,
			}, nil
		},
		function.WithName("sandbox_read_file"),
		function.WithDescription(sandboxToolDescription("读取当前会话 Sandbox 工作区中的文本文件。大文件请使用 offset 和 limit 分段读取。")),
	)
}

func (s *SandboxToolSet) writeFileTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxWriteFileInput) (sandboxFileOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			filePath, err := sandboxPath(input.Path, false)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			if _, err := workspace.Files.WriteString(ctx, filePath, input.Content, e2b.FsOptions{}); err != nil {
				return sandboxFileOutput{}, err
			}
			return sandboxFileOutput{Path: sandboxRelativePath(filePath), Message: "file saved"}, nil
		},
		function.WithName("sandbox_write_file"),
		function.WithDescription(sandboxToolDescription("在当前会话 Sandbox 工作区中创建或覆盖一个文本文件；缺失的父目录会自动创建。处理中写入 work/，最终交付文件写入 artifacts/。")),
	)
}

func (s *SandboxToolSet) makeDirectoryTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxPathInput) (sandboxFileOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			directory, err := sandboxPath(input.Path, false)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			if err := workspace.Files.MakeDir(ctx, directory, e2b.FsOptions{}); err != nil {
				return sandboxFileOutput{}, err
			}
			return sandboxFileOutput{Path: sandboxRelativePath(directory), Message: "directory created"}, nil
		},
		function.WithName("sandbox_make_directory"),
		function.WithDescription(sandboxToolDescription("在当前会话 Sandbox 工作区中创建目录，包括缺失的父目录。")),
	)
}

func (s *SandboxToolSet) movePathTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxMovePathInput) (sandboxFileOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			from, err := sandboxPath(input.From, false)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			to, err := sandboxPath(input.To, false)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			if err := workspace.Files.Move(ctx, from, to, e2b.FsOptions{}); err != nil {
				return sandboxFileOutput{}, err
			}
			return sandboxFileOutput{Path: sandboxRelativePath(to), Message: "path moved"}, nil
		},
		function.WithName("sandbox_move_path"),
		function.WithDescription(sandboxToolDescription("移动或重命名当前会话 Sandbox 工作区中的文件或目录。")),
	)
}

func (s *SandboxToolSet) deletePathTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxPathInput) (sandboxFileOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			filePath, err := sandboxPath(input.Path, false)
			if err != nil {
				return sandboxFileOutput{}, err
			}
			if err := workspace.Files.Remove(ctx, filePath, e2b.FsOptions{}); err != nil {
				return sandboxFileOutput{}, err
			}
			return sandboxFileOutput{Path: sandboxRelativePath(filePath), Message: "path deleted"}, nil
		},
		function.WithName("sandbox_delete_path"),
		function.WithDescription(sandboxToolDescription("删除当前会话 Sandbox 工作区中的文件或目录。")),
	)
}

func sandboxPath(input string, allowRoot bool) (string, error) {
	relative := strings.ReplaceAll(strings.TrimSpace(input), `\`, "/")
	if relative == "" || relative == "." {
		if allowRoot {
			return sandbox.Workspace.Root, nil
		}
		return "", fmt.Errorf("workspace root is not allowed here")
	}
	if strings.HasPrefix(relative, "/") {
		return "", fmt.Errorf("sandbox paths must be workspace-relative")
	}
	for _, part := range strings.Split(relative, "/") {
		if part == ".." {
			return "", fmt.Errorf("sandbox paths cannot contain '..'")
		}
	}
	return path.Join(sandbox.Workspace.Root, relative), nil
}

func sandboxRelativePath(realPath string) string {
	cleaned := path.Clean(realPath)
	if cleaned == sandbox.Workspace.Root {
		return "."
	}
	prefix := sandbox.Workspace.Root + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return realPath
	}
	return strings.TrimPrefix(cleaned, prefix)
}

func sandboxEntryType(entryType e2b.EntryType) string {
	if entryType == e2b.EntryTypeDirectory {
		return "directory"
	}
	return "file"
}
