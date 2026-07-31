// types.go 集中 cronjob 包的对外类型与常量。
package cronjob

import (
	"errors"
	"time"
)

const (
	// TaskTypeOnce 是一次性任务：执行完关闭，不再有下次。
	TaskTypeOnce = "once"
	// TaskTypeRecurring 是周期性任务：执行完按 cron 表达式计算下次。
	TaskTypeRecurring = "recurring"
)

var (
	// ErrInvalidJob 表示任务输入不合法（字段缺失或 cron 表达式无法解析）。
	ErrInvalidJob = errors.New("invalid cron job")
	// ErrNotFound 表示任务不存在或不属于该用户。
	ErrNotFound = errors.New("cron job not found")
)

// Job 是 cron_jobs 表的一行任务定义。
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

// JobInput 是可编辑的任务字段。ID 与用户归属由 Store 管理。
type JobInput struct {
	Name     string
	TaskType string
	Schedule string
	Prompt   string
}
