package sandbox

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type pathRequest struct {
	Path string `json:"path" jsonschema:"description=File or directory path (relative)"`
}

type readFileRequest struct {
	Path   string `json:"path" jsonschema:"description=Relative file path to read"`
	Offset *int   `json:"offset,omitempty" jsonschema:"description=Optional 0-based byte offset to start reading from"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"description=Optional max bytes to return"`
}

type writeFileRequest struct {
	Path    string `json:"path" jsonschema:"description=Relative file path to write"`
	Content string `json:"content" jsonschema:"description=Text content to write into the file"`
}

type listFileRequest struct {
	Path  string `json:"path" jsonschema:"description=Directory path to list, empty means root"`
	Depth int    `json:"depth,omitempty" jsonschema:"description=How many directory levels to descend, default 1"`
}

type moveFileRequest struct {
	From string `json:"from" jsonschema:"description=Source path"`
	To   string `json:"to" jsonschema:"description=Destination path"`
}

type fileResponse struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type readFileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Message string `json:"message"`
}

type listFileResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
	Message string      `json:"message"`
}

func (s *sandboxToolSet) readFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.readFile,
		function.WithName("file_read"),
		function.WithDescription("Read a file and return its contents."),
	)
}

func (s *sandboxToolSet) readFile(ctx context.Context, req readFileRequest) (readFileResponse, error) {
	rsp := readFileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	data, err := backend.ReadFile(ctx, req.Path)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}

	content := string(data)
	if req.Offset != nil || req.Limit != nil {
		content = sliceBytes(data, req.Offset, req.Limit)
	}
	rsp.Content = content
	rsp.Message = fmt.Sprintf("Successfully read %s (%d bytes)", req.Path, len(data))
	return rsp, nil
}

func (s *sandboxToolSet) writeFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.writeFile,
		function.WithName("file_write"),
		function.WithDescription("Create or overwrite a file with the given content."),
	)
}

func (s *sandboxToolSet) writeFile(ctx context.Context, req writeFileRequest) (fileResponse, error) {
	rsp := fileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	if err := backend.WriteFile(ctx, req.Path, []byte(req.Content)); err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Message = fmt.Sprintf("Successfully saved: %s", req.Path)
	return rsp, nil
}

func (s *sandboxToolSet) listFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.listFiles,
		function.WithName("file_list"),
		function.WithDescription("List files and directories. Set depth > 1 for recursive listing."),
	)
}

func (s *sandboxToolSet) listFiles(ctx context.Context, req listFileRequest) (listFileResponse, error) {
	rsp := listFileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	depth := req.Depth
	if depth <= 0 {
		depth = 1
	}
	entries, err := backend.ListDir(ctx, req.Path, depth)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Entries = entries
	rsp.Message = fmt.Sprintf("Found %d entries in %s", len(entries), req.Path)
	return rsp, nil
}

func (s *sandboxToolSet) makeDirTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.makeDir,
		function.WithName("file_mkdir"),
		function.WithDescription("Create a directory including any missing parent directories."),
	)
}

func (s *sandboxToolSet) makeDir(ctx context.Context, req pathRequest) (fileResponse, error) {
	rsp := fileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	if err := backend.MakeDir(ctx, req.Path); err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Message = fmt.Sprintf("Successfully created directory: %s", req.Path)
	return rsp, nil
}

func (s *sandboxToolSet) removeFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.removeFile,
		function.WithName("file_remove"),
		function.WithDescription("Remove a file or directory recursively."),
	)
}

func (s *sandboxToolSet) removeFile(ctx context.Context, req pathRequest) (fileResponse, error) {
	rsp := fileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	if err := backend.Remove(ctx, req.Path); err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Message = fmt.Sprintf("Successfully removed: %s", req.Path)
	return rsp, nil
}

func (s *sandboxToolSet) fileExistsTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.fileExists,
		function.WithName("file_exists"),
		function.WithDescription("Check whether a file or directory exists."),
	)
}

func (s *sandboxToolSet) fileExists(ctx context.Context, req pathRequest) (fileResponse, error) {
	rsp := fileResponse{Path: req.Path}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	exists, err := backend.Exists(ctx, req.Path)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	if exists {
		rsp.Message = fmt.Sprintf("Path exists: %s", req.Path)
	} else {
		rsp.Message = fmt.Sprintf("Path does not exist: %s", req.Path)
	}
	return rsp, nil
}

func (s *sandboxToolSet) moveFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.moveFile,
		function.WithName("file_move"),
		function.WithDescription("Move or rename a file or directory."),
	)
}

func (s *sandboxToolSet) moveFile(ctx context.Context, req moveFileRequest) (fileResponse, error) {
	rsp := fileResponse{Path: fmt.Sprintf("%s → %s", req.From, req.To)}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	if err := backend.Move(ctx, req.From, req.To); err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Message = fmt.Sprintf("Successfully moved: %s → %s", req.From, req.To)
	return rsp, nil
}

func sliceBytes(data []byte, offset, limit *int) string {
	start := 0
	if offset != nil && *offset > 0 {
		start = *offset
	}
	if start >= len(data) {
		return ""
	}
	end := len(data)
	if limit != nil && *limit > 0 && start+*limit < end {
		end = start + *limit
	}
	return string(data[start:end])
}
