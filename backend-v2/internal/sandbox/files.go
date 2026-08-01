package sandbox

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/eric642/e2b-go-sdk"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

const defaultReadLimit = 32 * 1024

type pathInput struct {
	Path string `json:"path" jsonschema:"description=Sandbox workspace-relative path"`
}
type listFilesInput struct {
	Path  string `json:"path,omitempty" jsonschema:"description=Workspace-relative directory"`
	Depth int    `json:"depth,omitempty" jsonschema:"description=Directory depth; default 1"`
}
type readFileInput struct {
	Path   string `json:"path" jsonschema:"description=Workspace-relative text file,required"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type writeFileInput struct {
	Path    string `json:"path" jsonschema:"description=Workspace-relative file,required"`
	Content string `json:"content" jsonschema:"description=Text content,required"`
}
type movePathInput struct {
	From string `json:"from" jsonschema:"description=Source path,required"`
	To   string `json:"to" jsonschema:"description=Destination path,required"`
}
type fileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}
type listFilesOutput struct {
	Path    string      `json:"path"`
	Entries []fileEntry `json:"entries"`
}
type readFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}
type fileOutput struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (s *toolSet) listFilesTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input listFilesInput) (listFilesOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return listFilesOutput{}, err
		}
		directory, err := workspacePath(input.Path, true)
		if err != nil {
			return listFilesOutput{}, err
		}
		depth := input.Depth
		if depth == 0 {
			depth = 1
		}
		entries, err := workspace.Files.List(ctx, directory, e2b.FsOptions{Depth: depth})
		if err != nil {
			return listFilesOutput{}, err
		}
		output := listFilesOutput{Path: relativePath(directory), Entries: []fileEntry{}}
		for _, entry := range entries {
			kind := "file"
			if entry.Type == e2b.EntryTypeDirectory {
				kind = "directory"
			}
			output.Entries = append(output.Entries, fileEntry{Name: entry.Name, Path: relativePath(entry.Path), Type: kind, Size: entry.Size})
		}
		return output, nil
	}, function.WithName("sandbox_list_files"), function.WithDescription(toolDescription("列出当前会话 Sandbox 工作区中的文件和目录。")))
}
func (s *toolSet) readFileTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input readFileInput) (readFileOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return readFileOutput{}, err
		}
		filePath, err := workspacePath(input.Path, false)
		if err != nil {
			return readFileOutput{}, err
		}
		if input.Offset < 0 {
			return readFileOutput{}, fmt.Errorf("file offset cannot be negative")
		}
		limit := input.Limit
		if limit == 0 {
			limit = defaultReadLimit
		}
		if limit < 1 {
			return readFileOutput{}, fmt.Errorf("file limit must be positive")
		}
		reader, err := workspace.Files.ReadStream(ctx, filePath, e2b.FsOptions{})
		if err != nil {
			return readFileOutput{}, err
		}
		defer reader.Close()
		if input.Offset > 0 {
			if _, err := io.CopyN(io.Discard, reader, int64(input.Offset)); err != nil && err != io.EOF {
				return readFileOutput{}, err
			}
		}
		data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
		if err != nil {
			return readFileOutput{}, err
		}
		truncated := len(data) > limit
		if truncated {
			data = data[:limit]
		}
		return readFileOutput{Path: relativePath(filePath), Content: string(data), Truncated: truncated}, nil
	}, function.WithName("sandbox_read_file"), function.WithDescription(toolDescription("读取当前会话 Sandbox 工作区中的文本文件。")))
}
func (s *toolSet) writeFileTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input writeFileInput) (fileOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return fileOutput{}, err
		}
		filePath, err := workspacePath(input.Path, false)
		if err != nil {
			return fileOutput{}, err
		}
		if _, err := workspace.Files.WriteString(ctx, filePath, input.Content, e2b.FsOptions{}); err != nil {
			return fileOutput{}, err
		}
		return fileOutput{Path: relativePath(filePath), Message: "file saved"}, nil
	}, function.WithName("sandbox_write_file"), function.WithDescription(toolDescription("在当前会话 Sandbox 中创建或覆盖文本文件。")))
}
func (s *toolSet) makeDirectoryTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input pathInput) (fileOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return fileOutput{}, err
		}
		directory, err := workspacePath(input.Path, false)
		if err != nil {
			return fileOutput{}, err
		}
		if err := workspace.Files.MakeDir(ctx, directory, e2b.FsOptions{}); err != nil {
			return fileOutput{}, err
		}
		return fileOutput{Path: relativePath(directory), Message: "directory created"}, nil
	}, function.WithName("sandbox_make_directory"), function.WithDescription(toolDescription("创建目录，包括缺失的父目录。")))
}
func (s *toolSet) movePathTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input movePathInput) (fileOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return fileOutput{}, err
		}
		from, err := workspacePath(input.From, false)
		if err != nil {
			return fileOutput{}, err
		}
		to, err := workspacePath(input.To, false)
		if err != nil {
			return fileOutput{}, err
		}
		if err := workspace.Files.Move(ctx, from, to, e2b.FsOptions{}); err != nil {
			return fileOutput{}, err
		}
		return fileOutput{Path: relativePath(to), Message: "path moved"}, nil
	}, function.WithName("sandbox_move_path"), function.WithDescription(toolDescription("移动或重命名当前会话 Sandbox 中的文件或目录。")))
}
func (s *toolSet) deletePathTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input pathInput) (fileOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return fileOutput{}, err
		}
		filePath, err := workspacePath(input.Path, false)
		if err != nil {
			return fileOutput{}, err
		}
		if err := workspace.Files.Remove(ctx, filePath, e2b.FsOptions{}); err != nil {
			return fileOutput{}, err
		}
		return fileOutput{Path: relativePath(filePath), Message: "path deleted"}, nil
	}, function.WithName("sandbox_delete_path"), function.WithDescription(toolDescription("删除当前会话 Sandbox 中的文件或目录。")))
}
func workspacePath(input string, allowRoot bool) (string, error) {
	relative := strings.ReplaceAll(strings.TrimSpace(input), `\\`, "/")
	if relative == "" || relative == "." {
		if allowRoot {
			return Workspace.Root, nil
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
	return path.Join(Workspace.Root, relative), nil
}
func relativePath(realPath string) string {
	cleaned := path.Clean(realPath)
	if cleaned == Workspace.Root {
		return "."
	}
	return strings.TrimPrefix(cleaned, Workspace.Root+"/")
}
