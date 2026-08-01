package agentrun

import (
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

type runOptionInput struct {
	requestID         string
	modelID           string
	apiKey            string
	globalInstruction string
	additionalTools   []tool.Tool
}

// frameworkRunOptions 把一次运行的数据显式转换为框架选项。
func frameworkRunOptions(input runOptionInput) []agent.RunOption {
	return []agent.RunOption{
		agent.WithRequestID(input.requestID),
		agent.WithStream(true),
		agent.WithModelName(input.modelID),
		agent.WithModelRequestHeaders(map[string]string{"Authorization": "Bearer " + input.apiKey}),
		agent.WithGlobalInstruction(input.globalInstruction),
		agent.WithInstruction("需要知道当前时间时，调用 get_current_time 工具。"),
		agent.WithAdditionalTools(input.additionalTools),
	}
}
