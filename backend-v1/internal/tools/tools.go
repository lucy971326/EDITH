// Package tools contains EDITH's built-in system tools.
//
// These tools are shared by every user. User-specific MCP and sandbox tools
// are created per run and belong in agent.WithAdditionalTools instead.
package tools

import "trpc.group/trpc-go/trpc-agent-go/tool"

// Default is the complete built-in tool surface for EDITH 1.0.
var Default = []tool.Tool{
	GetCurrentTime,
}
