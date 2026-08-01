package cronjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"edith/backend-v2/internal/userconfig"

	"github.com/google/uuid"
)

// store 是 cronjob 模块私有的数据访问器；外部只能使用 HTTP、Tools 和 Scheduler。
type store struct {
	db       *sql.DB
	settings *userconfig.Settings
}

// Create 创建任务并计算首次执行时间。
func (s *store) Create(ctx context.Context, userID string, input JobInput) (Job, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Job{}, fmt.Errorf("%w: user ID is required", ErrInvalidJob)
	}
	if err := validateInput(input); err != nil {
		return Job{}, err
	}
	now := time.Now()
	next, err := s.nextRun(ctx, userID, input, now)
	if err != nil {
		return Job{}, err
	}
	job := Job{
		ID: uuid.NewString(), ClerkUserID: userID,
		Name: strings.TrimSpace(input.Name), TaskType: input.TaskType,
		Schedule: strings.TrimSpace(input.Schedule), Prompt: strings.TrimSpace(input.Prompt),
		Enabled: true, NextRunAt: next, CreatedAt: now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO cron_jobs (
			id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		) VALUES (?, ?, ?, ?, ?, ?, 1, ?, 0, ?)
	`, job.ID, job.ClerkUserID, job.Name, job.TaskType, job.Schedule,
		job.Prompt, databaseTimePtr(job.NextRunAt), databaseTime(job.CreatedAt))
	if err != nil {
		return Job{}, fmt.Errorf("create cron job: %w", err)
	}
	return job, nil
}

// List 返回一个用户的任务，供任务管理页展示。
func (s *store) List(ctx context.Context, userID string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs WHERE clerk_user_id = ? ORDER BY created_at, id
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list cron jobs: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read cron jobs: %w", err)
	}
	return jobs, nil
}

// Update 替换任务可编辑字段并重新计算下次执行时间。
func (s *store) Update(ctx context.Context, userID, jobID string, input JobInput) (Job, error) {
	if err := validateInput(input); err != nil {
		return Job{}, err
	}
	next, err := s.nextRun(ctx, strings.TrimSpace(userID), input, time.Now())
	if err != nil {
		return Job{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE cron_jobs
		SET name = ?, task_type = ?, schedule = ?, prompt = ?, next_run_at = ?
		WHERE id = ? AND clerk_user_id = ?
	`, strings.TrimSpace(input.Name), input.TaskType, strings.TrimSpace(input.Schedule),
		strings.TrimSpace(input.Prompt), databaseTimePtr(next),
		strings.TrimSpace(jobID), strings.TrimSpace(userID))
	if err != nil {
		return Job{}, fmt.Errorf("update cron job: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return Job{}, ErrNotFound
	}
	return s.job(ctx, strings.TrimSpace(jobID))
}

// Delete 删除属于当前用户的任务。
func (s *store) Delete(ctx context.Context, userID, jobID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = ? AND clerk_user_id = ?`, strings.TrimSpace(jobID), strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// SetEnabled 切换任务开关；重新启用已完成的一次性任务会按原时间计算，通常应由用户编辑新时间。
func (s *store) SetEnabled(ctx context.Context, userID, jobID string, enabled bool) (Job, error) {
	current, err := s.jobForUser(ctx, strings.TrimSpace(userID), strings.TrimSpace(jobID))
	if err != nil {
		return Job{}, err
	}
	next := current.NextRunAt
	if enabled && next == nil {
		next, err = s.nextRun(ctx, current.ClerkUserID, JobInput{
			Name: current.Name, TaskType: current.TaskType,
			Schedule: current.Schedule, Prompt: current.Prompt,
		}, time.Now())
		if err != nil {
			return Job{}, err
		}
	}
	value := 0
	if enabled {
		value = 1
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE cron_jobs SET enabled = ?, next_run_at = ?
		WHERE id = ? AND clerk_user_id = ?
	`, value, databaseTimePtr(next), current.ID, current.ClerkUserID)
	if err != nil {
		return Job{}, fmt.Errorf("set cron job enabled: %w", err)
	}
	return s.job(ctx, current.ID)
}

func (s *store) jobForUser(ctx context.Context, userID, jobID string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs WHERE id = ? AND clerk_user_id = ?
	`, jobID, userID)
	return jobFromRow(row)
}
func (s *store) job(ctx context.Context, jobID string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule,
			prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs WHERE id = ?
	`, jobID)
	return jobFromRow(row)
}

type scanner interface{ Scan(...any) error }

func jobFromRow(row scanner) (Job, error) { return readJob(row) }
func readJob(row scanner) (Job, error) {
	var job Job
	var enabled, running int
	var next sql.NullString
	var created string
	if err := row.Scan(&job.ID, &job.ClerkUserID, &job.Name, &job.TaskType, &job.Schedule, &job.Prompt, &enabled, &next, &running, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	job.Enabled, job.Running = enabled == 1, running == 1
	if next.Valid {
		value, err := time.Parse(time.RFC3339, next.String)
		if err != nil {
			return Job{}, fmt.Errorf("parse next run: %w", err)
		}
		job.NextRunAt = &value
	}
	value, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return Job{}, fmt.Errorf("parse created time: %w", err)
	}
	job.CreatedAt = value
	return job, nil
}
func databaseTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
func databaseTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return databaseTime(*value)
}
