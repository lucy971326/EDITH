package tools

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"edith/backend-v1/internal/cronjob"
	"edith/backend-v1/internal/userconfig"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func TestCronJobToolUsesInvocationUser(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	users, err := userconfig.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := cronjob.New(db, users)
	if err != nil {
		t.Fatal(err)
	}

	var createTool interface {
		Call(context.Context, []byte) (any, error)
	}
	for _, candidate := range (&CronJobToolSet{CronJobs: store}).Tools(t.Context()) {
		if candidate.Declaration().Name == "create_cron_job" {
			createTool, _ = candidate.(interface {
				Call(context.Context, []byte) (any, error)
			})
			break
		}
	}
	if createTool == nil {
		t.Fatal("create_cron_job was not registered")
	}

	invocation := agent.NewInvocation(agent.WithInvocationSession(&session.Session{
		UserID: "clerk_alice",
		ID:     "chat-1",
	}))
	ctx := agent.NewInvocationContext(context.Background(), invocation)
	_, err = createTool.Call(ctx, []byte(`{
		"name":"提醒",
		"taskType":"once",
		"schedule":"2026-08-01T09:30:00+08:00",
		"prompt":"提醒我"
	}`))
	if err != nil {
		t.Fatal(err)
	}

	aliceJobs, err := store.List(context.Background(), "clerk_alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceJobs) != 1 || aliceJobs[0].ClerkUserID != "clerk_alice" {
		t.Fatalf("alice jobs = %#v", aliceJobs)
	}
	bobJobs, err := store.List(context.Background(), "clerk_bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(bobJobs) != 0 {
		t.Fatalf("bob jobs = %#v", bobJobs)
	}
	_, err = createTool.Call(context.Background(), []byte(`{
		"name":"没有身份",
		"taskType":"once",
		"schedule":"2026-08-01T09:30:00+08:00",
		"prompt":"不应该创建"
	}`))
	if err == nil {
		t.Fatal("create_cron_job accepted a context without an invocation")
	}
}
