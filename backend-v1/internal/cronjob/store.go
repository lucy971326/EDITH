// Package cronjob 提供 EDITH 的定时任务能力：任务定义存储、调度触发与执行收尾。
// 设计结论：配置真相在 cron_jobs 表，运行真相在 ManagedRunner，
// 结果真相在会话历史（cron:<job_id> 的 session），因此没有执行记录表。
package cronjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"edith/backend-v1/internal/userconfig"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Store 持有 cron_jobs 表的读写与调度所需的原子操作。
// users 用于读取用户时区，解释 cron 表达式与一次性任务时间。
type Store struct {
	db    *sql.DB
	users *userconfig.Store
}

// New 创建 cron_jobs 表并返回存储。
func New(db *sql.DB, users *userconfig.Store) (*Store, error) {
	if db == nil {
		return nil, errors.New("cron job database is required")
	}
	if users == nil {
		return nil, errors.New("cron job user config store is required")
	}
	store := &Store{db: db, users: users}
	if err := store.createTables(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) createTables(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cron_jobs (
			id TEXT PRIMARY KEY,
			clerk_user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			task_type TEXT NOT NULL,
			schedule TEXT NOT NULL,
			prompt TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			next_run_at TEXT,
			running INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create cron jobs table: %w", err)
	}
	return nil
}

// Create 校验输入并创建任务，初始 next_run_at 按用户时区计算。
func (s *Store) Create(ctx context.Context, userID string, input JobInput) (Job, error) {
	userID = strings.TrimSpace(userID)
	if err := validateInput(input); err != nil {
		return Job{}, err
	}
	now := time.Now()
	next, err := s.nextRun(ctx, userID, input, now)
	if err != nil {
		return Job{}, err
	}
	job := Job{
		ID:          uuid.NewString(),
		ClerkUserID: userID,
		Name:        strings.TrimSpace(input.Name),
		TaskType:    input.TaskType,
		Schedule:    strings.TrimSpace(input.Schedule),
		Prompt:      strings.TrimSpace(input.Prompt),
		Enabled:     true,
		NextRunAt:   next,
		CreatedAt:   now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO cron_jobs (id, clerk_user_id, name, task_type, schedule, prompt, enabled, next_run_at, running, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, 0, ?)
	`, job.ID, job.ClerkUserID, job.Name, job.TaskType, job.Schedule, job.Prompt, formatTimePtr(job.NextRunAt), formatTime(job.CreatedAt))
	if err != nil {
		return Job{}, fmt.Errorf("create cron job %q: %w", job.ID, err)
	}
	return job, nil
}

// List 返回一个用户的全部任务，按创建时间排序。
func (s *Store) List(ctx context.Context, userID string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule, prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs
		WHERE clerk_user_id = ?
		ORDER BY created_at, id
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list cron jobs %q: %w", userID, err)
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cron jobs %q: %w", userID, err)
	}
	return jobs, nil
}

// Update 校验输入并更新任务字段，同时按新配置重算 next_run_at。
func (s *Store) Update(ctx context.Context, userID, jobID string, input JobInput) (Job, error) {
	userID = strings.TrimSpace(userID)
	jobID = strings.TrimSpace(jobID)
	if userID == "" || jobID == "" {
		return Job{}, ErrNotFound
	}
	if err := validateInput(input); err != nil {
		return Job{}, err
	}
	next, err := s.nextRun(ctx, userID, input, time.Now())
	if err != nil {
		return Job{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE cron_jobs
		SET name = ?, task_type = ?, schedule = ?, prompt = ?, next_run_at = ?
		WHERE id = ? AND clerk_user_id = ?
	`, strings.TrimSpace(input.Name), input.TaskType, strings.TrimSpace(input.Schedule), strings.TrimSpace(input.Prompt), formatTimePtr(next), jobID, userID)
	if err != nil {
		return Job{}, fmt.Errorf("update cron job %q: %w", jobID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Job{}, ErrNotFound
	}
	return s.get(ctx, userID, jobID)
}

// Delete 删除一个属于该用户的任务。
func (s *Store) Delete(ctx context.Context, userID, jobID string) error {
	userID = strings.TrimSpace(userID)
	jobID = strings.TrimSpace(jobID)
	if userID == "" || jobID == "" {
		return ErrNotFound
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = ? AND clerk_user_id = ?`, jobID, userID)
	if err != nil {
		return fmt.Errorf("delete cron job %q: %w", jobID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return ErrNotFound
	}
	return nil
}

// SetEnabled 启用或停用任务。启用且没有 next_run_at 时初始化首次执行时间。
func (s *Store) SetEnabled(ctx context.Context, userID, jobID string, enabled bool) (Job, error) {
	userID = strings.TrimSpace(userID)
	jobID = strings.TrimSpace(jobID)
	if userID == "" || jobID == "" {
		return Job{}, ErrNotFound
	}
	var next any
	if enabled {
		current, err := s.get(ctx, userID, jobID)
		if err != nil {
			return Job{}, err
		}
		if current.NextRunAt == nil {
			computed, err := s.nextRun(ctx, userID, JobInput{Name: current.Name, TaskType: current.TaskType, Schedule: current.Schedule, Prompt: current.Prompt}, time.Now())
			if err != nil {
				return Job{}, err
			}
			next = formatTimePtr(computed)
		} else {
			next = formatTimePtr(current.NextRunAt)
		}
	}
	value := 0
	if enabled {
		value = 1
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE cron_jobs SET enabled = ?, next_run_at = COALESCE(?, next_run_at)
		WHERE id = ? AND clerk_user_id = ?
	`, value, next, jobID, userID)
	if err != nil {
		return Job{}, fmt.Errorf("set cron job enabled %q: %w", jobID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Job{}, ErrNotFound
	}
	return s.get(ctx, userID, jobID)
}

// DueJobs 返回当前到点且未在运行的任务，供调度器抢占。
func (s *Store) DueJobs(ctx context.Context, now time.Time) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule, prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs
		WHERE enabled = 1 AND running = 0 AND next_run_at IS NOT NULL AND next_run_at <= ?
		ORDER BY next_run_at
	`, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("list due cron jobs: %w", err)
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due cron jobs: %w", err)
	}
	return jobs, nil
}

// ClaimDue 原子抢占一个到点任务：影响行数为 1 才算抢到。
// 抢占成功后任务进入 running=1，同一任务不会被第二个调度器或下一次轮询重复触发。
func (s *Store) ClaimDue(ctx context.Context, jobID string, now time.Time) (Job, bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE cron_jobs SET running = 1
		WHERE id = ? AND enabled = 1 AND running = 0
			AND next_run_at IS NOT NULL AND next_run_at <= ?
	`, jobID, formatTime(now))
	if err != nil {
		return Job{}, false, fmt.Errorf("claim cron job %q: %w", jobID, err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Job{}, false, nil
	}
	job, err := s.get(ctx, "", jobID)
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

// FinishRun 在任务收尾（事件流结束）时释放 running，并按 task_type 安排下次。
// 一次性任务关闭；周期性任务按用户时区计算下一次执行时间。
func (s *Store) FinishRun(ctx context.Context, jobID string, now time.Time) error {
	job, err := s.get(ctx, "", jobID)
	if err != nil {
		return err
	}
	var next *time.Time
	if job.TaskType == TaskTypeRecurring {
		computed, err := s.nextRun(ctx, job.ClerkUserID, JobInput{Name: job.Name, TaskType: job.TaskType, Schedule: job.Schedule, Prompt: job.Prompt}, now)
		if err != nil {
			return err
		}
		next = computed
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE cron_jobs
		SET running = 0,
			next_run_at = ?,
			enabled = CASE WHEN task_type = ? THEN 0 ELSE enabled END
		WHERE id = ?
	`, formatTimePtr(next), TaskTypeOnce, jobID)
	if err != nil {
		return fmt.Errorf("finish cron job %q: %w", jobID, err)
	}
	return nil
}

// InitializeNextRuns 为已启用但没有 next_run_at 的任务补上首次执行时间。
// 用于新任务创建后启用，以及调度器启动时的数据修复。
func (s *Store) InitializeNextRuns(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, clerk_user_id, name, task_type, schedule, prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs
		WHERE enabled = 1 AND next_run_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("list cron jobs without next run: %w", err)
	}
	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate cron jobs without next run: %w", err)
	}
	for _, job := range jobs {
		next, err := s.nextRun(ctx, job.ClerkUserID, JobInput{Name: job.Name, TaskType: job.TaskType, Schedule: job.Schedule, Prompt: job.Prompt}, now)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET next_run_at = ? WHERE id = ?`, formatTimePtr(next), job.ID); err != nil {
			return fmt.Errorf("initialize next run %q: %w", job.ID, err)
		}
	}
	return nil
}

func (s *Store) get(ctx context.Context, userID, jobID string) (Job, error) {
	query := `
		SELECT id, clerk_user_id, name, task_type, schedule, prompt, enabled, next_run_at, running, created_at
		FROM cron_jobs WHERE id = ?`
	args := []any{jobID}
	if userID != "" {
		query += " AND clerk_user_id = ?"
		args = append(args, userID)
	}
	row := s.db.QueryRowContext(ctx, query, args...)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

// nextRun 计算任务的下一次执行时间。
// 一次性任务：schedule 存 RFC3339 时间戳；周期性任务：schedule 存标准 5 段 cron 表达式，按用户时区解释。
func (s *Store) nextRun(ctx context.Context, userID string, input JobInput, from time.Time) (*time.Time, error) {
	schedule := strings.TrimSpace(input.Schedule)
	if input.TaskType == TaskTypeOnce {
		parsed, err := time.Parse(time.RFC3339, schedule)
		if err != nil {
			return nil, fmt.Errorf("%w: once schedule must be RFC3339 time", ErrInvalidJob)
		}
		return &parsed, nil
	}
	timezone, err := s.users.LoadTimezone(ctx, userID)
	if err != nil {
		return nil, err
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown timezone %q", ErrInvalidJob, timezone)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	scheduleParser, err := parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cron expression", ErrInvalidJob)
	}
	// Next 按传入时间的时区解释字段，因此先转换到用户时区再计算。
	next := scheduleParser.Next(from.In(location))
	return &next, nil
}

func validateInput(input JobInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidJob)
	}
	if input.TaskType != TaskTypeOnce && input.TaskType != TaskTypeRecurring {
		return fmt.Errorf("%w: taskType must be once or recurring", ErrInvalidJob)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return fmt.Errorf("%w: prompt is required", ErrInvalidJob)
	}
	if strings.TrimSpace(input.Schedule) == "" {
		return fmt.Errorf("%w: schedule is required", ErrInvalidJob)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var (
		job        Job
		enabled    int
		running    int
		nextRunRaw sql.NullString
		createdRaw string
	)
	if err := row.Scan(&job.ID, &job.ClerkUserID, &job.Name, &job.TaskType, &job.Schedule, &job.Prompt, &enabled, &nextRunRaw, &running, &createdRaw); err != nil {
		return Job{}, err
	}
	job.Enabled = enabled == 1
	job.Running = running == 1
	if nextRunRaw.Valid {
		parsed, err := time.Parse(time.RFC3339, nextRunRaw.String)
		if err != nil {
			return Job{}, fmt.Errorf("parse cron job next run %q: %w", job.ID, err)
		}
		job.NextRunAt = &parsed
	}
	created, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return Job{}, fmt.Errorf("parse cron job created at %q: %w", job.ID, err)
	}
	job.CreatedAt = created
	return job, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
