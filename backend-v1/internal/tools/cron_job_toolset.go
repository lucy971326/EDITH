package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"edith/backend-v1/internal/cronjob"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// CronJobToolSet 是 EDITH 的定时任务工具集。
// Store 由 main 创建并注入；当前用户从 Runner Invocation 中读取。
type CronJobToolSet struct {
	CronJobs *cronjob.Store
}

type createCronJobInput struct {
	Name     string `json:"name" description:"任务名称，简短描述任务用途"`
	TaskType string `json:"taskType" description:"任务类型，只能是 once（一次性）或 recurring（周期性）"`
	Schedule string `json:"schedule" description:"一次性任务填写 RFC3339 时间；周期性任务填写 5 段 cron 表达式"`
	Prompt   string `json:"prompt" description:"任务到点后交给 Agent 执行的指令"`
}

type createCronJobOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TaskType  string `json:"taskType"`
	Schedule  string `json:"schedule"`
	NextRunAt string `json:"nextRunAt"`
	Message   string `json:"message"`
}

// Tools 注册当前 ToolSet 提供的定时任务工具。
func (s *CronJobToolSet) Tools(context.Context) []tool.Tool {
	return []tool.Tool{s.createCronJobTool()}
}

// Close 实现 tool.ToolSet；任务存储由 main 创建并统一管理生命周期。
func (s *CronJobToolSet) Close() error { return nil }

// Name 返回 ToolSet 名称。
func (s *CronJobToolSet) Name() string { return "cronjob" }

func (s *CronJobToolSet) createCronJobTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input createCronJobInput) (createCronJobOutput, error) {
			if s.CronJobs == nil {
				return createCronJobOutput{}, fmt.Errorf("cron job tools have no cron job store")
			}

			invocation, ok := agent.InvocationFromContext(ctx)
			if !ok || invocation == nil || invocation.Session == nil {
				return createCronJobOutput{}, fmt.Errorf("cron job tools require a Runner invocation with a session")
			}
			userID := strings.TrimSpace(invocation.Session.UserID)
			if userID == "" {
				return createCronJobOutput{}, fmt.Errorf("cron job tools require a Runner session user")
			}

			job, err := s.CronJobs.Create(ctx, userID, cronjob.JobInput{
				Name:     input.Name,
				TaskType: input.TaskType,
				Schedule: input.Schedule,
				Prompt:   input.Prompt,
			})
			if err != nil {
				return createCronJobOutput{}, err
			}

			nextRunAt := ""
			if job.NextRunAt != nil {
				nextRunAt = job.NextRunAt.Format(time.RFC3339)
			}
			return createCronJobOutput{
				ID:        job.ID,
				Name:      job.Name,
				TaskType:  job.TaskType,
				Schedule:  job.Schedule,
				NextRunAt: nextRunAt,
				Message:   "定时任务已创建",
			}, nil
		},
		function.WithName("create_cron_job"),
		function.WithDescription("创建一个定时任务。只能填写任务名称、任务类型、执行时间或 cron 表达式、任务指令；用户身份由当前 Runner 会话自动确定。"),
	)
}
