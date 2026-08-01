package systemtools

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type currentTimeInput struct {
	Timezone string `json:"timezone" description:"时区，例如 Asia/Shanghai"`
}

type currentTimeOutput struct {
	Time     string `json:"time" description:"当前时间，格式 YYYY-MM-DD HH:MM:SS"`
	Timezone string `json:"timezone" description:"返回的时区"`
}

func currentTimeTool() tool.Tool {
	return function.NewFunctionTool(
		func(_ context.Context, input currentTimeInput) (currentTimeOutput, error) {
			location, err := time.LoadLocation(input.Timezone)
			if err != nil {
				return currentTimeOutput{}, fmt.Errorf("load timezone %q: %w", input.Timezone, err)
			}
			return currentTimeOutput{
				Time: time.Now().In(location).Format("2006-01-02 15:04:05"), Timezone: input.Timezone,
			}, nil
		},
		function.WithName("get_current_time"),
		function.WithDescription("获取指定时区的当前时间。用户询问现在几点、某地时间或需要比较时区时使用。"),
	)
}
