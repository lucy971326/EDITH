package sandbox

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// NewToolSet creates the sandbox tools exposed to the Agent.
func NewToolSet(provider BackendProvider) tool.ToolSet {
	toolSet := &sandboxToolSet{provider: provider}
	toolSet.tools = []tool.Tool{
		toolSet.readFileTool(),
		toolSet.writeFileTool(),
		toolSet.listFileTool(),
		toolSet.makeDirTool(),
		toolSet.removeFileTool(),
		toolSet.fileExistsTool(),
		toolSet.moveFileTool(),
		toolSet.runCommandTool(),
	}
	return toolSet
}

// sandboxToolSet keeps the tool list and the provider used to find each
// invocation's isolated execution backend.
type sandboxToolSet struct {
	provider BackendProvider
	tools    []tool.Tool
}

func (s *sandboxToolSet) Tools(context.Context) []tool.Tool { return s.tools }
func (s *sandboxToolSet) Close() error                      { return s.provider.Close() }
func (s *sandboxToolSet) Name() string                      { return "sandbox" }

// getBackend reads the current user and session from the Runner invocation,
// then asks the provider for that workspace's backend.
func (s *sandboxToolSet) getBackend(ctx context.Context) (ExecBackend, error) {
	invocation, ok := agent.InvocationFromContext(ctx)
	if !ok || invocation == nil || invocation.Session == nil {
		return nil, fmt.Errorf("sandbox tools require a Runner invocation with a session")
	}

	return s.provider.GetBackend(ctx, WorkspaceID{
		UserID:    invocation.Session.UserID,
		SessionID: invocation.Session.ID,
	})
}
