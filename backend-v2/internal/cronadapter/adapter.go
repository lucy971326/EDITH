// Package cronadapter 把定时任务转换为 Gateway 消息。
package cronadapter

import (
	"context"
	"errors"
	"fmt"

	"edith/backend-v2/internal/cronjob"
	"edith/backend-v2/internal/gateway"

	"github.com/google/uuid"
)

// Adapter 是 Cron 渠道执行器；它不负责轮询和任务存储。
type Adapter struct {
	gateway *gateway.Service
}

// New 创建 CronAdapter；Gateway 必须由 main 显式提供。
func New(agentGateway *gateway.Service) (*Adapter, error) {
	if agentGateway == nil {
		return nil, errors.New("cronadapter requires a gateway")
	}
	return &Adapter{gateway: agentGateway}, nil
}

// RunJob 把一个已抢占任务交给 Gateway，并消费事件直到 AgentRun 收尾。
func (a *Adapter) RunJob(_ context.Context, job cronjob.Job) error {
	stream, runError := a.gateway.Run(gateway.IncomingMessage{
		Channel: gateway.CronChannel, ExternalUserID: job.ClerkUserID,
		SessionKey: "cron:" + job.ID, RequestID: uuid.NewString(), Message: job.Prompt,
	})
	if runError != nil {
		return fmt.Errorf("start cron agent run: %s", runError.Message)
	}
	var executionError error
	for streamEvent := range stream.Events {
		if streamEvent.Type == "run.error" && streamEvent.Error != nil {
			executionError = errors.New(streamEvent.Error.Message)
		}
	}
	return executionError
}
