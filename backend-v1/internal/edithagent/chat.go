// Package edithagent declares EDITH's long-lived Agent skeleton.
package edithagent

import (
	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/tools"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
)

// Chat is the single shared chat agent for EDITH.
//
// It deliberately has no default prompt, user API key, user MCP, user skills,
// or sandbox. Those are all per-run differences injected through RunOptions.
var Chat = llmagent.New("edith-chat",
	llmagent.WithModels(models.Registered),
	llmagent.WithModel(models.MiniMaxM3),
	llmagent.WithTools(tools.Default),
)
