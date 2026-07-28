// Package tools contains EDITH's built-in system tools.
//
// This file is the registry for EDITH's default tool surface. MCP tools do not
// belong here: they come from a user's configuration and are added per Run.
package tools

import (
	"edith/backend-v1/internal/sandbox"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Defaults is EDITH's complete built-in tool registry. Tools and ToolSets are
// separate only because trpc-agent-go registers them through different options.
type Defaults struct {
	Tools    []tool.Tool
	ToolSets []tool.ToolSet
}

// Default returns every built-in EDITH tool. SandboxToolSet resolves its user
// and session from the current invocation, never from main.
func Default(sandboxes *sandbox.Service) Defaults {
	return Defaults{
		Tools: []tool.Tool{
			GetCurrentTime,
		},
		ToolSets: []tool.ToolSet{
			&SandboxToolSet{Sandboxes: sandboxes},
		},
	}
}
