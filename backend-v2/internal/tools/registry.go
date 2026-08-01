// Package tools 汇总各功能模块提供的 Agent 工具。
package tools

import "trpc.group/trpc-go/trpc-agent-go/tool"

// Dependencies 是 main 显式交给工具注册表的能力。
type Dependencies struct {
	Tools    []tool.Tool
	ToolSets []tool.ToolSet
}

// Registry 是创建共享 Agent 时使用的工具清单。
type Registry struct {
	Tools    []tool.Tool
	ToolSets []tool.ToolSet
}

// New 汇总工具，不创建任何业务工具。
func New(deps Dependencies) *Registry {
	return &Registry{
		Tools:    append([]tool.Tool(nil), deps.Tools...),
		ToolSets: append([]tool.ToolSet(nil), deps.ToolSets...),
	}
}
