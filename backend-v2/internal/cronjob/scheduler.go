package cronjob

import (
	"context"
	"log"
	"time"
)

const DefaultPollInterval = 10 * time.Second

// JobRunner 是调度器启动一次任务所需的唯一边界；cronadapter 实现它。
type JobRunner interface {
	RunJob(context.Context, Job) error
}

// Scheduler 周期扫描到点任务；由 main 显式调用 Run 启动。
type Scheduler struct {
	jobs     *store
	runner   JobRunner
	interval time.Duration
}

// Run 持续调度到 ctx 取消；每个任务都在独立 goroutine 中运行。
func (s *Scheduler) Run(ctx context.Context) {
	if err := s.jobs.initializeNextRuns(ctx, time.Now()); err != nil {
		log.Printf("initialize cron next runs: %v", err)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.scheduleDueJobs(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// scheduleDueJobs 原子抢占本轮到点任务，并交给 JobRunner 执行。
func (s *Scheduler) scheduleDueJobs(ctx context.Context) {
	now := time.Now()
	jobs, err := s.jobs.dueJobs(ctx, now)
	if err != nil {
		log.Printf("load due cron jobs: %v", err)
		return
	}
	for _, job := range jobs {
		claimed, acquired, err := s.jobs.claimDue(ctx, job.ID, now)
		if err != nil {
			log.Printf("claim cron job %q: %v", job.ID, err)
			continue
		}
		if !acquired {
			continue
		}
		go s.runClaimedJob(claimed)
	}
}

// runClaimedJob 确保任务无论执行成败都会释放 running 状态。
func (s *Scheduler) runClaimedJob(job Job) {
	// 已抢占任务不继承轮询 ctx；停止 Scheduler 只阻止新任务进入。
	taskContext := context.Background()
	if err := s.runner.RunJob(taskContext, job); err != nil {
		log.Printf("run cron job %q: %v", job.ID, err)
	}
	if err := s.jobs.finishRun(taskContext, job.ID, time.Now()); err != nil {
		log.Printf("finish cron job %q: %v", job.ID, err)
	}
}
