package cronjob

import (
	"context"
	"fmt"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// toolSet 是 cronjob 自己提供给 Agent 的工具集合。
type toolSet struct{ jobs *store }

func (s *toolSet) Name() string                      { return "cronjob" }
func (s *toolSet) Close() error                      { return nil }
func (s *toolSet) Tools(context.Context) []tool.Tool { return []tool.Tool{s.createJobTool()} }

type createJobInput struct {
	Name     string `json:"name" description:"任务名称，简短描述任务用途"`
	TaskType string `json:"taskType" description:"任务类型：once 或 recurring"`
	Schedule string `json:"schedule" description:"一次性任务填写 RFC3339 时间；周期任务填写五段 cron 表达式"`
	Prompt   string `json:"prompt" description:"到点后交给 Agent 执行的指令"`
}
type createJobOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TaskType  string `json:"taskType"`
	Schedule  string `json:"schedule"`
	NextRunAt string `json:"nextRunAt"`
	Message   string `json:"message"`
}

// createJobTool 创建当前 Runner 用户的任务；用户身份不属于模型输入。
func (s *toolSet) createJobTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input createJobInput) (createJobOutput, error) {
			invocation, ok := agent.InvocationFromContext(ctx)
			if !ok || invocation == nil || invocation.Session == nil {
				return createJobOutput{}, fmt.Errorf("cronjob tool requires a Runner invocation with a session")
			}
			userID := strings.TrimSpace(invocation.Session.UserID)
			if userID == "" {
				return createJobOutput{}, fmt.Errorf("cronjob tool requires a Runner session user")
			}
			job, err := s.jobs.Create(ctx, userID, JobInput{Name: input.Name, TaskType: input.TaskType, Schedule: input.Schedule, Prompt: input.Prompt})
			if err != nil {
				return createJobOutput{}, err
			}
			next := ""
			if job.NextRunAt != nil {
				next = job.NextRunAt.Format(time.RFC3339)
			}
			return createJobOutput{ID: job.ID, Name: job.Name, TaskType: job.TaskType, Schedule: job.Schedule, NextRunAt: next, Message: "定时任务已创建"}, nil
		},
		function.WithName("create_cron_job"),
		function.WithDescription("创建当前用户的定时任务。只能填写任务名称、类型、执行时间和任务指令；用户身份由当前 Runner 会话自动确定。"),
	)
}
