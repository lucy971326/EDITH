package cronjob

import (
	"context"
	"log"
	"time"
)

// DefaultPollInterval 是调度器扫描数据库的默认间隔。
const DefaultPollInterval = 10 * time.Second

// Scheduler 轮询 cron_jobs 表，把到点且未被抢占的任务交给 Adapter 执行。
// 单实例部署：防双跑依赖 ClaimDue 的原子 UPDATE，不引入分布式锁。
type Scheduler struct {
	store    *Store
	adapter  JobRunner
	interval time.Duration
}

// JobRunner 是调度器执行一次定时任务所需的最小能力。
// 具体的 Agent 渠道适配由 cronadapter 包提供，避免存储层依赖 Gateway。
type JobRunner interface {
	RunJob(context.Context, Job) error
}

// NewScheduler 创建调度器，默认每 10 秒轮询一次。
func NewScheduler(store *Store, adapter JobRunner) *Scheduler {
	return &Scheduler{store: store, adapter: adapter, interval: DefaultPollInterval}
}

// WithInterval 覆盖轮询间隔，主要供测试使用。
func (s *Scheduler) WithInterval(interval time.Duration) *Scheduler {
	if interval > 0 {
		s.interval = interval
	}
	return s
}

// Run 启动调度循环，直到 ctx 取消。
// 启动时先为缺失 next_run_at 的任务补上首次执行时间。
func (s *Scheduler) Run(ctx context.Context) {
	if err := s.store.InitializeNextRuns(ctx, time.Now()); err != nil {
		log.Printf("initialize cron next runs: %v", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// tick 找出到点任务并逐个原子抢占；抢占成功后在后台执行。
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()
	jobs, err := s.store.DueJobs(ctx, now)
	if err != nil {
		log.Printf("load due cron jobs: %v", err)
		return
	}
	for _, job := range jobs {
		claimed, ok, err := s.store.ClaimDue(ctx, job.ID, now)
		if err != nil {
			log.Printf("claim cron job %q: %v", job.ID, err)
			continue
		}
		if !ok {
			// 已被其他轮询抢占或任务被停用，跳过。
			continue
		}
		go s.runJob(ctx, claimed)
	}
}

// runJob 执行任务并收尾：无论成功失败都释放 running 并安排下次执行。
func (s *Scheduler) runJob(ctx context.Context, job Job) {
	if err := s.adapter.RunJob(ctx, job); err != nil {
		log.Printf("run cron job %q: %v", job.ID, err)
	}
	if err := s.store.FinishRun(ctx, job.ID, time.Now()); err != nil {
		log.Printf("finish cron job %q: %v", job.ID, err)
	}
}
