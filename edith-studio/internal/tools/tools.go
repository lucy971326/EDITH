// Package tools 统一注册 Studio 使用的 Agent 工具。
package tools

import (
	"fmt"

	"edith/studio/internal/claudecode"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// NewToolSets 为当前项目创建 Claude Code 兼容工具集。
func NewToolSets(projectRoot string) ([]tool.ToolSet, error) {
	toolSet, err := claudecode.NewToolSet(claudecode.WithBaseDir(projectRoot))
	if err != nil {
		return nil, fmt.Errorf("create Claude Code tool set: %w", err)
	}
	return []tool.ToolSet{toolSet}, nil
}
