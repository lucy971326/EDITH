package cronjob

import (
	"context"
	"fmt"
	"time"
)

// dueJobs 找出本次轮询可能执行的任务；真正的互斥由 claimDue 保证。
func (s *store) dueJobs(ctx context.Context, now time.Time) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs
		WHERE enabled = 1 AND running = 0
			AND next_run_at IS NOT NULL AND next_run_at <= ?
		ORDER BY next_run_at
	`, databaseTime(now))
	if err != nil {
		return nil, fmt.Errorf("list due cron jobs: %w", err)
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		job, err := readJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// claimDue 原子占用任务；返回 false 表示已被其他调度循环抢走。
func (s *store) claimDue(ctx context.Context, jobID string, now time.Time) (Job, bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE cron_jobs SET running = 1
		WHERE id = ? AND enabled = 1 AND running = 0
			AND next_run_at IS NOT NULL AND next_run_at <= ?
	`, jobID, databaseTime(now))
	if err != nil {
		return Job{}, false, fmt.Errorf("claim cron job: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return Job{}, false, nil
	}
	job, err := s.job(ctx, jobID)
	return job, err == nil, err
}

// finishRun 释放任务占用，并按任务类型写入下一次执行时间。
func (s *store) finishRun(ctx context.Context, jobID string, now time.Time) error {
	job, err := s.job(ctx, jobID)
	if err != nil {
		return err
	}
	var next *time.Time
	if job.TaskType == TaskTypeRecurring {
		next, err = s.nextRun(ctx, job.ClerkUserID, JobInput{
			Name: job.Name, TaskType: job.TaskType,
			Schedule: job.Schedule, Prompt: job.Prompt,
		}, now)
		if err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE cron_jobs
		SET running = 0, next_run_at = ?,
			enabled = CASE WHEN task_type = ? THEN 0 ELSE enabled END
		WHERE id = ?
	`, databaseTimePtr(next), TaskTypeOnce, jobID)
	if err != nil {
		return fmt.Errorf("finish cron job: %w", err)
	}
	return nil
}

// initializeNextRuns 补齐启用但未安排时间的任务，供 Scheduler 启动时调用。
func (s *store) initializeNextRuns(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs WHERE enabled = 1 AND next_run_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("list cron jobs without next run: %w", err)
	}
	jobs := []Job{}
	for rows.Next() {
		job, err := readJob(rows)
		if err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read cron jobs without next run: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close cron jobs without next run: %w", err)
	}

	// SQLite 允许单连接部署；释放查询 rows 后才能执行后续 UPDATE。
	for _, job := range jobs {
		next, err := s.nextRun(ctx, job.ClerkUserID, JobInput{
			Name: job.Name, TaskType: job.TaskType,
			Schedule: job.Schedule, Prompt: job.Prompt,
		}, now)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE cron_jobs SET next_run_at = ? WHERE id = ?`,
			databaseTimePtr(next), job.ID,
		); err != nil {
			return fmt.Errorf("set initial cron run: %w", err)
		}
	}
	return nil
}
