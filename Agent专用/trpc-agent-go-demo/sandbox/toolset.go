package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// ============================================================================
// SandboxToolSet — wraps an ExecBackend into a tool.ToolSet for Agent use.
// ============================================================================

// NewToolSet creates a ToolSet from the given ExecBackend.
func NewToolSet(backend ExecBackend) tool.ToolSet {
	ts := &sandboxToolSet{
		backend: backend,
		name:    "sandbox",
	}
	ts.init()
	return ts
}

type sandboxToolSet struct {
	backend ExecBackend
	tools   []tool.Tool
	name    string
}

func (s *sandboxToolSet) init() {
	s.tools = []tool.Tool{
		s.readFileTool(),
		s.writeFileTool(),
		s.listFileTool(),
		s.makeDirTool(),
		s.fileRemoveTool(),
		s.fileExistsTool(),
		s.fileMoveTool(),
		s.runCommandTool(),
	}
}

func (s *sandboxToolSet) Tools(ctx context.Context) []tool.Tool { return s.tools }
func (s *sandboxToolSet) Close() error                          { return s.backend.Close() }
func (s *sandboxToolSet) Name() string                          { return s.name }

// ============================================================================
// Request / Response types
// ============================================================================

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

type renameRequest struct {
	From string `json:"from" jsonschema:"description=Source path"`
	To   string `json:"to" jsonschema:"description=Destination path"`
}

type runCommandRequest struct {
	Command string            `json:"command" jsonschema:"description=Shell command or program to run"`
	Args    []string          `json:"args,omitempty" jsonschema:"description=Command arguments"`
	Envs    map[string]string `json:"envs,omitempty" jsonschema:"description=Environment variables"`
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

type runCommandResponse struct {
	Command  string `json:"command"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
}

// ============================================================================
// Tool implementations
// ============================================================================

func (s *sandboxToolSet) readFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req readFileRequest) (readFileResponse, error) {
			rsp := readFileResponse{Path: req.Path}
			data, err := s.backend.ReadFile(ctx, req.Path)
			if err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			result := string(data)
			if req.Offset != nil || req.Limit != nil {
				result = sliceBytes(data, req.Offset, req.Limit)
			}
			rsp.Content = result
			rsp.Message = fmt.Sprintf("Successfully read %s (%d bytes)", req.Path, len(data))
			return rsp, nil
		},
		function.WithName("file_read"),
		function.WithDescription("Read a file and return its contents."),
	)
}

func (s *sandboxToolSet) writeFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req writeFileRequest) (fileResponse, error) {
			rsp := fileResponse{Path: req.Path}
			if err := s.backend.WriteFile(ctx, req.Path, []byte(req.Content)); err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Message = fmt.Sprintf("Successfully saved: %s", req.Path)
			return rsp, nil
		},
		function.WithName("file_write"),
		function.WithDescription("Create or overwrite a file with the given content."),
	)
}

func (s *sandboxToolSet) listFileTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req listFileRequest) (listFileResponse, error) {
			rsp := listFileResponse{Path: req.Path}
			depth := req.Depth
			if depth <= 0 {
				depth = 1
			}
			entries, err := s.backend.ListDir(ctx, req.Path, depth)
			if err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Entries = entries
			rsp.Message = fmt.Sprintf("Found %d entries in %s", len(entries), req.Path)
			return rsp, nil
		},
		function.WithName("file_list"),
		function.WithDescription("List files and directories. Set depth > 1 for recursive listing."),
	)
}

func (s *sandboxToolSet) makeDirTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req pathRequest) (fileResponse, error) {
			rsp := fileResponse{Path: req.Path}
			if err := s.backend.MakeDir(ctx, req.Path); err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Message = fmt.Sprintf("Successfully created directory: %s", req.Path)
			return rsp, nil
		},
		function.WithName("file_mkdir"),
		function.WithDescription("Create a directory including any missing parent directories."),
	)
}

func (s *sandboxToolSet) fileRemoveTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req pathRequest) (fileResponse, error) {
			rsp := fileResponse{Path: req.Path}
			if err := s.backend.Remove(ctx, req.Path); err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Message = fmt.Sprintf("Successfully removed: %s", req.Path)
			return rsp, nil
		},
		function.WithName("file_remove"),
		function.WithDescription("Remove a file or directory recursively."),
	)
}

func (s *sandboxToolSet) fileExistsTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req pathRequest) (fileResponse, error) {
			rsp := fileResponse{Path: req.Path}
			ok, err := s.backend.Exists(ctx, req.Path)
			if err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			if ok {
				rsp.Message = fmt.Sprintf("Path exists: %s", req.Path)
			} else {
				rsp.Message = fmt.Sprintf("Path does not exist: %s", req.Path)
			}
			return rsp, nil
		},
		function.WithName("file_exists"),
		function.WithDescription("Check whether a file or directory exists."),
	)
}

func (s *sandboxToolSet) fileMoveTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req renameRequest) (fileResponse, error) {
			rsp := fileResponse{Path: fmt.Sprintf("%s → %s", req.From, req.To)}
			if err := s.backend.Move(ctx, req.From, req.To); err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Message = fmt.Sprintf("Successfully moved: %s → %s", req.From, req.To)
			return rsp, nil
		},
		function.WithName("file_move"),
		function.WithDescription("Move or rename a file or directory."),
	)
}

func (s *sandboxToolSet) runCommandTool() tool.CallableTool {
	return function.NewFunctionTool(
		func(ctx context.Context, req runCommandRequest) (runCommandResponse, error) {
			rsp := runCommandResponse{Command: req.Command}
			result, err := s.backend.RunCommand(ctx, req.Command, req.Args, req.Envs)
			if err != nil {
				rsp.Message = fmt.Sprintf("Error: %v", err)
				return rsp, err
			}
			rsp.Stdout = result.Stdout
			rsp.Stderr = result.Stderr
			rsp.ExitCode = result.ExitCode
			if result.ExitCode == 0 {
				rsp.Message = "Command completed successfully."
			} else {
				rsp.Message = fmt.Sprintf("Command exited with code %d.", result.ExitCode)
			}
			return rsp, nil
		},
		function.WithName("run_command"),
		function.WithDescription(
			"Execute a program in the sandbox. "+
				"Use 'command' for the program name (e.g. 'whoami', 'cat', 'ls') "+
				"and 'args' for its arguments (e.g. ['/etc/os-release']). "+
				"For complex operations, 'args' can include '-c' and a shell expression "+
				"when 'command' is '/bin/sh' or '/bin/bash'.",
		),
	)
}

// ============================================================================
// Helpers
// ============================================================================

func sliceBytes(data []byte, offset, limit *int) string {
	start := 0
	if offset != nil && *offset > 0 {
		start = *offset
	}
	if start >= len(data) {
		return ""
	}
	end := len(data)
	if limit != nil && *limit > 0 {
		if start+*limit < end {
			end = start + *limit
		}
	}
	return string(data[start:end])
}

// Ensure json is used (for any future response marshaling needs).
var _ = json.Marshal
