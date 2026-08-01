// Command server starts EDITH's private Go runtime for the Web BFF.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"edith/backend-v1/internal/cronadapter"
	"edith/backend-v1/internal/cronjob"
	"edith/backend-v1/internal/gateway"
	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/sandbox"
	"edith/backend-v1/internal/tools"
	"edith/backend-v1/internal/usage"
	"edith/backend-v1/internal/userconfig"
	"edith/backend-v1/internal/webadapter"
	"edith/backend-v1/internal/webapi"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

const appName = "EDITH"

func main() {
	loadEnv()

	// EDITH-owned services share one SQLite handle. The framework session
	// service owns a separate handle because its Close method closes the DB it
	// receives. Both handles use WAL plus a busy timeout for SQLite writes.
	appDB, err := openDatabase(databasePath())
	if err != nil {
		log.Fatalf("open EDITH database: %v", err)
	}
	defer appDB.Close()

	users, err := userconfig.Open(appDB)
	if err != nil {
		log.Fatalf("open user config store: %v", err)
	}

	sessionDB, err := openDatabase(databasePath())
	if err != nil {
		log.Fatalf("open session database: %v", err)
	}
	rawSessions, err := sessionsqlite.NewService(sessionDB)
	if err != nil {
		sessionDB.Close()
		log.Fatalf("open session service: %v", err)
	}
	defer rawSessions.Close()

	// chatImages owns chat-image metadata, ownership checks, and private COS access.
	chatImages, err := images.Open(appDB, imageConfig())
	if err != nil {
		log.Fatalf("open image service: %v", err)
	}
	imageSessions := images.WrapSessionService(rawSessions, chatImages)

	err = usage.CreateTable(appDB)
	if err != nil {
		log.Fatalf("create usage tables: %v", err)
	}

	sandboxes, err := sandbox.Open(appDB, sandboxTemplate())
	if err != nil {
		log.Fatalf("open sandbox service: %v", err)
	}

	cronStore, err := cronjob.New(appDB, users)
	if err != nil {
		log.Fatalf("create cron job store: %v", err)
	}

	defaultTools := tools.Default(sandboxes, cronStore)
	edithAgent := llmagent.New(
		"edith-chat",
		llmagent.WithModels(models.Registered),
		llmagent.WithModel(models.Registered[models.DefaultModelID]),
		llmagent.WithTools(defaultTools.Tools),
		llmagent.WithToolSets(defaultTools.ToolSets),
	)

	edithRunner := runner.NewRunner(
		appName,
		edithAgent,
		runner.WithSessionService(imageSessions),
	)

	managedRunner, ok := edithRunner.(runner.ManagedRunner)
	if !ok {
		log.Fatal("EDITH runner does not support run control")
	}
	agentGateway, err := gateway.New(managedRunner, users, chatImages, appDB)
	if err != nil {
		log.Fatalf("create agent gateway: %v", err)
	}
	webAdapter, err := webadapter.New(agentGateway)
	if err != nil {
		log.Fatalf("create web adapter: %v", err)
	}
	cronAdapter, err := cronadapter.New(agentGateway)
	if err != nil {
		log.Fatalf("create cron job adapter: %v", err)
	}
	cronScheduler := cronjob.NewScheduler(cronStore, cronAdapter)

	webapi := webapi.Server{
		AppName:  appName,
		Users:    users,
		CronJobs: cronStore,
		Images:   chatImages,
		Sessions: rawSessions,
		UsageDB:  appDB,
	}
	mux := http.NewServeMux()
	webAdapter.Register(mux)
	webapi.Register(mux)

	address := runtimeAddress()
	log.Printf("EDITH runtime listening on http://%s", address)

	httpServer := http.Server{Addr: address, Handler: mux}

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 定时任务调度器随进程启动，收到退出信号时停止轮询。
	go cronScheduler.Run(shutdown)
	go func() {
		<-shutdown.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown HTTP server: %v", err)
		}
	}()

	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func openDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// One connection per owner keeps SQLite write ordering simple. WAL permits
	// readers while a writer is active; busy_timeout turns short lock races into
	// waiting instead of immediate "database is locked" failures.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// loadEnv loads local development configuration. Existing process environment
// variables take priority, so deployment configuration can override .env.
func loadEnv() {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load .env: %v", err)
	}
}

func databasePath() string {
	if path := os.Getenv("EDITH_DB_PATH"); path != "" {
		return path
	}
	return "edith.db"
}

func runtimeAddress() string {
	if address := os.Getenv("EDITH_ADDR"); address != "" {
		return address
	}
	return "127.0.0.1:8080"
}

func sandboxTemplate() string {
	template := os.Getenv("EDITH_E2B_TEMPLATE")
	if template == "" {
		log.Fatal("EDITH_E2B_TEMPLATE is required")
	}
	return template
}

func imageConfig() images.Config {
	return images.Config{
		Bucket:    os.Getenv("EDITH_COS_BUCKET"),
		Region:    os.Getenv("EDITH_COS_REGION"),
		SecretID:  os.Getenv("EDITH_COS_SECRET_ID"),
		SecretKey: os.Getenv("EDITH_COS_SECRET_KEY"),
	}
}
