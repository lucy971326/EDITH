// Package runopts translates EDITH's per-run data into framework RunOptions.
package runopts

import (
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Config contains only data for one Agent run. It is not a long-lived service
// configuration and must not be stored on the shared Agent or Runner.
type Config struct {
	RequestID         string
	Stream            bool
	ModelName         string
	APIKey            string
	GlobalInstruction string
	Instruction       string
	AdditionalTools   []tool.Tool
}

// Build returns EDITH's seven core RunOptions in one visible place.
//
// ModelName and APIKey come from server-side user configuration. APIKey enters
// only the request header for this run; shared model adapters never retain it.
func Build(config Config) []agent.RunOption {
	return []agent.RunOption{
		agent.WithRequestID(config.RequestID),
		agent.WithStream(config.Stream),
		agent.WithModelName(config.ModelName),
		agent.WithModelRequestHeaders(map[string]string{
			"Authorization": "Bearer " + config.APIKey,
		}),
		agent.WithGlobalInstruction(config.GlobalInstruction),
		agent.WithInstruction(config.Instruction),
		agent.WithAdditionalTools(config.AdditionalTools),
	}
}
