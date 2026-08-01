package cronjob

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"edith/backend-v1/internal/userconfig"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	// 与生产 main.go 一致：单连接串行化 SQLite 写入。
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	users, err := userconfig.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db, users)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestCreateRejectsInvalidCron(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "bad", TaskType: TaskTypeRecurring, Schedule: "not-a-cron", Prompt: "hi",
	})
	if !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("err = %v, want ErrInvalidJob", err)
	}
}

func TestCreateOnceJobInitializesNextRun(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "one-shot", TaskType: TaskTypeOnce, Schedule: "2026-08-01T09:30:00+08:00", Prompt: "提醒我",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.NextRunAt == nil {
		t.Fatal("once job has no next run")
	}
	if job.NextRunAt.UTC().Format(time.RFC3339) != "2026-08-01T01:30:00Z" {
		t.Fatalf("next run = %v, want 2026-08-01T01:30:00Z", job.NextRunAt)
	}
	if !job.Enabled || job.Running {
		t.Fatalf("job = %#v", job)
	}
}

func TestCreateRecurringJobUsesUserTimezone(t *testing.T) {
	store := newTestStore(t)
	if err := store.users.SaveTimezone(context.Background(), "clerk_1", "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "daily", TaskType: TaskTypeRecurring, Schedule: "0 9 * * *", Prompt: "晨报",
	})
	if err != nil {
		t.Fatal(err)
	}
	// now = 2026-07-31T10:00+08:00，下一个 9 点应是 2026-08-01T09:00+08:00 = 01:00Z。
	now := time.Date(2026, 7, 31, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	next, err := store.nextRun(context.Background(), "clerk_1", JobInput{Name: job.Name, TaskType: job.TaskType, Schedule: job.Schedule, Prompt: job.Prompt}, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.UTC().Format(time.RFC3339) != "2026-08-01T01:00:00Z" {
		t.Fatalf("next run = %v, want 2026-08-01T01:00:00Z", next)
	}
}

func TestClaimDueIsAtomic(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "due", TaskType: TaskTypeOnce, Schedule: "2020-01-01T00:00:00Z", Prompt: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var winners sync.Map
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := store.ClaimDue(context.Background(), job.ID, now)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if ok {
				winners.Store("won", true)
			}
		}()
	}
	wg.Wait()
	if _, won := winners.Load("won"); !won {
		t.Fatal("no goroutine won the claim")
	}
	// 抢占后任务处于 running，任何重复抢占都失败。
	_, ok, err := store.ClaimDue(context.Background(), job.ID, now)
	if err != nil || ok {
		t.Fatalf("second claim ok = %t, err = %v", ok, err)
	}
}

func TestFinishRunOnceDisablesJob(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "one", TaskType: TaskTypeOnce, Schedule: "2020-01-01T00:00:00Z", Prompt: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.ClaimDue(context.Background(), job.ID, time.Now()); err != nil || !ok {
		t.Fatalf("claim ok = %t, err = %v", ok, err)
	}
	if err := store.FinishRun(context.Background(), job.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	done, err := store.get(context.Background(), "", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Enabled || done.Running || done.NextRunAt != nil {
		t.Fatalf("finished once job = %#v", done)
	}
}

func TestFinishRunRecurringAdvancesNextRun(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "daily", TaskType: TaskTypeRecurring, Schedule: "0 9 * * *", Prompt: "晨报",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 把下次执行时间改为过去，模拟到点任务。
	if _, err := store.db.Exec(`UPDATE cron_jobs SET next_run_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", job.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.ClaimDue(context.Background(), job.ID, time.Now()); err != nil || !ok {
		t.Fatalf("claim ok = %t, err = %v", ok, err)
	}
	// 用已过当天 9 点的时间收尾，下一次应推到次日 9 点。
	finishAt := time.Date(2026, 8, 1, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	if err := store.FinishRun(context.Background(), job.ID, finishAt); err != nil {
		t.Fatal(err)
	}
	after, err := store.get(context.Background(), "", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Running || after.NextRunAt == nil {
		t.Fatalf("finished recurring job = %#v", after)
	}
	if after.NextRunAt.UTC().Format(time.RFC3339) != "2026-08-02T01:00:00Z" {
		t.Fatalf("next run = %v, want 2026-08-02T01:00:00Z", after.NextRunAt)
	}
}

func TestSetEnabledAndUpdate(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Create(context.Background(), "clerk_1", JobInput{
		Name: "daily", TaskType: TaskTypeRecurring, Schedule: "0 9 * * *", Prompt: "晨报",
	})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := store.SetEnabled(context.Background(), "clerk_1", job.ID, false)
	if err != nil || disabled.Enabled {
		t.Fatalf("disabled = %#v, err = %v", disabled, err)
	}
	due, err := store.DueJobs(context.Background(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range due {
		if item.ID == job.ID {
			t.Fatal("disabled job still due")
		}
	}
	updated, err := store.Update(context.Background(), "clerk_1", job.ID, JobInput{
		Name: "renamed", TaskType: TaskTypeRecurring, Schedule: "30 8 * * *", Prompt: "改过的晨报",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "renamed" || updated.Schedule != "30 8 * * *" || updated.Prompt != "改过的晨报" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestListScopedToUser(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Create(context.Background(), "clerk_1", JobInput{Name: "a", TaskType: TaskTypeOnce, Schedule: "2026-08-01T00:00:00Z", Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), "clerk_2", JobInput{Name: "b", TaskType: TaskTypeOnce, Schedule: "2026-08-02T00:00:00Z", Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	jobs, err := store.List(context.Background(), "clerk_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ClerkUserID != "clerk_1" {
		t.Fatalf("jobs = %#v", jobs)
	}
}
