// Package tools 统一注册 Studio 使用的 Agent 工具。
package tools

import (
	"fmt"

	"edith/studio/internal/claudecode"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent/builtin"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	agenttool "trpc.group/trpc-go/trpc-agent-go/tool/agent"
)

// NewToolSets 为当前项目创建 Claude Code 兼容工具集。
func NewToolSets(projectRoot string) ([]tool.ToolSet, error) {
	toolSet, err := claudecode.NewToolSet(claudecode.WithBaseDir(projectRoot))
	if err != nil {
		return nil, fmt.Errorf("create Claude Code tool set: %w", err)
	}
	return []tool.ToolSet{toolSet}, nil
}

// NewAgentTool 把框架内置只读探索 Agent 包装成可挂载的 AgentTool。
// 子 Agent 默认继承父工具面与模型，只读为软约束（instruction 提示），
// 子事件以 ParentMetadata 非 nil + Author=explorer 透传进父事件流。
// WithPinModel 清掉父继承的 ModelName，让 explorer 用自己的模型实例，
// 避免"deepseek-flash not found for agent explorer"的降级 WARN。
func NewAgentTool() tool.Tool {
	return agenttool.NewTool(
		builtin.NewExplorer(),
		agenttool.WithStreamInner(true),
		agenttool.WithPinModel(true),
	)
}
