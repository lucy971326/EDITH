// Package systemtools 提供不依赖用户数据的系统工具。
package systemtools

import "trpc.group/trpc-go/trpc-agent-go/tool"

// Module 是可安全共享给所有 Agent Run 的系统工具集合。
type Module struct {
	Tools []tool.Tool
}

// New 创建无状态系统工具。
func New() *Module {
	return &Module{Tools: []tool.Tool{currentTimeTool()}}
}
