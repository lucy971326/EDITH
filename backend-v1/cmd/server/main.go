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

	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/sandbox"
	"edith/backend-v1/internal/tools"
	"edith/backend-v1/internal/userconfig"
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

	// Both stores use different tables in the same SQLite file. Each service
	// owns its own connection and closes it when the process stops.
	users, err := userconfig.Open(databasePath())
	if err != nil {
		log.Fatalf("open user config store: %v", err)
	}
	defer users.Close()

	sessionDB, err := sql.Open("sqlite3", databasePath())
	if err != nil {
		log.Fatalf("open session database: %v", err)
	}
	sessions, err := sessionsqlite.NewService(sessionDB)
	if err != nil {
		sessionDB.Close()
		log.Fatalf("open session service: %v", err)
	}
	defer sessions.Close()

	sandboxes, err := sandbox.Open(databasePath(), sandboxTemplate())
	if err != nil {
		log.Fatalf("open sandbox service: %v", err)
	}
	defer sandboxes.Close()

	defaultTools := tools.Default(sandboxes)
	chat := llmagent.New(
		"edith-chat",
		llmagent.WithModels(models.Registered),
		llmagent.WithModel(models.MiniMaxM3),
		llmagent.WithTools(defaultTools.Tools),
		llmagent.WithToolSets(defaultTools.ToolSets),
	)

	edithRunner := runner.NewRunner(
		appName,
		chat,
		runner.WithSessionService(sessions),
	)

	webapi := webapi.Server{
		AppName:  appName,
		Runner:   edithRunner,
		Users:    users,
		Sessions: sessions,
	}
	mux := http.NewServeMux()
	webapi.Register(mux)

	address := runtimeAddress()
	log.Printf("EDITH runtime listening on http://%s", address)

	httpServer := http.Server{Addr: address, Handler: mux}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
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
