package cronjob

import (
	"context"
	"database/sql"
	"errors"

	"edith/backend-v2/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Dependencies 是创建 cronjob 模块需要的外部能力。
type Dependencies struct {
	DB       *sql.DB
	Settings *userconfig.Settings
}

// Module 是定时任务功能的公开入口。
type Module struct {
	store *store
	Tools tool.ToolSet
	HTTP  *HTTP
}

// New 创建 cron_jobs 表和定时任务模块；不会启动调度协程。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("cronjob requires a database")
	}
	if deps.Settings == nil {
		return nil, errors.New("cronjob requires user settings")
	}
	if err := createSchema(context.Background(), deps.DB); err != nil {
		return nil, err
	}

	jobs := &store{db: deps.DB, settings: deps.Settings}
	module := &Module{store: jobs}
	module.Tools = &toolSet{jobs: jobs}
	module.HTTP = &HTTP{jobs: jobs, settings: deps.Settings}
	return module, nil
}

// NewScheduler 创建调度器；main 在进程启动时显式调用 Scheduler.Run。
func (m *Module) NewScheduler(runner JobRunner) (*Scheduler, error) {
	if runner == nil {
		return nil, errors.New("cronjob scheduler requires a job runner")
	}
	return &Scheduler{jobs: m.store, runner: runner, interval: DefaultPollInterval}, nil
}
