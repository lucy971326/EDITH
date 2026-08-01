// Package cronjob 提供用户定时任务的存储、HTTP 管理、Agent 工具和调度能力。
package cronjob

import (
	"errors"
	"time"
)

const (
	TaskTypeOnce      = "once"
	TaskTypeRecurring = "recurring"
)

var (
	ErrInvalidJob = errors.New("invalid cron job")
	ErrNotFound   = errors.New("cron job not found")
)

// Job 是一条定时任务定义；执行结果保存在对应的 Agent 会话历史中。
type Job struct {
	ID          string
	ClerkUserID string
	Name        string
	TaskType    string
	Schedule    string
	Prompt      string
	Enabled     bool
	NextRunAt   *time.Time
	Running     bool
	CreatedAt   time.Time
}

// JobInput 是创建或编辑任务时允许填写的字段。
type JobInput struct {
	Name     string
	TaskType string
	Schedule string
	Prompt   string
}

// CreateRequest 是创建任务的 HTTP 输入；UserID 由 BFF 注入，不由浏览器决定。
type CreateRequest struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	TaskType string `json:"taskType"`
	Schedule string `json:"schedule"`
	Prompt   string `json:"prompt"`
	Timezone string `json:"timezone"`
}

// UpdateRequest 是更新任务的 HTTP 输入。
type UpdateRequest = CreateRequest

// JobResponse 是返回给 Web 的安全任务表示。
type JobResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	TaskType  string  `json:"taskType"`
	Schedule  string  `json:"schedule"`
	Prompt    string  `json:"prompt"`
	Enabled   bool    `json:"enabled"`
	NextRunAt *string `json:"nextRunAt"`
	Running   bool    `json:"running"`
	CreatedAt string  `json:"createdAt"`
}

// ListResponse 是任务列表 HTTP 输出。
type ListResponse struct {
	Jobs []JobResponse `json:"jobs"`
}
