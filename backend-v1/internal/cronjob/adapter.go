package cronjob

import (
	"context"
	"errors"
	"fmt"

	"edith/backend-v1/internal/gateway"

	"github.com/google/uuid"
)

// Adapter 是定时任务的执行器，与 WebAdapter 对称：进程内调用 Gateway，
// 复用统一的 IncomingMessage 契约，不直接接触 OnlyRun 或 ManagedRunner。
type Adapter struct {
	agentGateway *gateway.Gateway
}

// NewAdapter 创建定时任务执行器。
func NewAdapter(agentGateway *gateway.Gateway) (*Adapter, error) {
	if agentGateway == nil {
		return nil, errors.New("cron adapter gateway is required")
	}
	return &Adapter{agentGateway: agentGateway}, nil
}

// RunJob 启动一次定时任务并消费完整事件流。
// 输入：一个已被调度器抢占的任务。
// 输出：事件流结束即任务收尾；启动失败返回错误，由调用方负责 FinishRun。
// 执行结果不在此返回，用户通过 cron:<job_id> 会话查看历史。
func (a *Adapter) RunJob(ctx context.Context, job Job) error {
	stream, apiError := a.agentGateway.Run(gateway.IncomingMessage{
		Channel:        "cron",
		ExternalUserID: job.ClerkUserID,
		SessionKey:     "cron:" + job.ID,
		RequestID:      uuid.NewString(),
		Message:        job.Prompt,
	})
	if apiError != nil {
		return fmt.Errorf("start cron run for job %q: %s", job.ID, apiError.Message)
	}
	// 定时任务不做流式展示；读完整个事件流等于等到任务真正收尾，
	// 之后才能安全释放 running 锁。
	for range stream.Events {
	}
	return nil
}
