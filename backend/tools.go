package main

import (
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// loadTools 创建 EDITH 自定义工具，并汇总其他组件暴露的工具。
func loadTools(componentTools ...[]tool.Tool) []tool.Tool {
	timeTool := function.NewFunctionTool(
		getCurrentTime,
		function.WithName("get_current_time"),
		function.WithDescription("查询指定时区当前的本地时间。不传 timezone 默认用北京时间。"),
	)

	tools := []tool.Tool{timeTool}
	for _, items := range componentTools {
		tools = append(tools, items...)
	}
	return tools
}
