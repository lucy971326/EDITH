package tools

import (
	"context"
	"fmt"

	"edith/backend-v1/internal/sandbox"

	"github.com/eric642/e2b-go-sdk"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const sandboxWorkspacePath = "/home/user"

const sandboxStateKey = "edith:sandbox-workspace"

// SandboxToolSet is EDITH's built-in filesystem and command capability. Its
// tools resolve the current user and session from the Agent invocation, so the
// model never receives a sandbox ID or another user's workspace.
type SandboxToolSet struct {
	Sandboxes *sandbox.Service
}

// Tools implements tool.ToolSet.
func (s *SandboxToolSet) Tools(context.Context) []tool.Tool {
	return []tool.Tool{
		s.listFilesTool(),
		s.readFileTool(),
		s.writeFileTool(),
		s.makeDirectoryTool(),
		s.movePathTool(),
		s.deletePathTool(),
		s.runCommandTool(),
		s.startProcessTool(),
		s.listProcessesTool(),
		s.killProcessTool(),
	}
}

// Close implements tool.ToolSet. Sandbox lifecycle belongs to sandbox.Service.
func (s *SandboxToolSet) Close() error { return nil }

// Name implements tool.ToolSet.
func (s *SandboxToolSet) Name() string { return "sandbox" }

func (s *SandboxToolSet) workspace(ctx context.Context) (*e2b.Sandbox, error) {
	if s.Sandboxes == nil {
		return nil, fmt.Errorf("sandbox tools have no sandbox service")
	}

	invocation, ok := agent.InvocationFromContext(ctx)
	if !ok || invocation == nil || invocation.Session == nil {
		return nil, fmt.Errorf("sandbox tools require a Runner invocation with a session")
	}

	if value, ok := invocation.GetState(sandboxStateKey); ok {
		workspace, ok := value.(*e2b.Sandbox)
		if !ok {
			return nil, fmt.Errorf("sandbox invocation state has an unexpected value")
		}
		return workspace, nil
	}

	workspace, err := s.Sandboxes.OpenWorkspace(ctx, invocation.Session.UserID, invocation.Session.ID)
	if err != nil {
		return nil, err
	}
	invocation.SetState(sandboxStateKey, workspace)
	return workspace, nil
}
