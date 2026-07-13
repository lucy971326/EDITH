// Command edith 启动 EDITH 单进程后端。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github-agent/edith/internal/config"
	"github-agent/edith/internal/httpserver"
	"github-agent/edith/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "edith ", log.LstdFlags|log.Lmsgprefix)
	logger.Printf("starting")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config load failed: %v", err)
	}

	if cfg.ClerkSecretKey != "" {
		clerk.SetKey(cfg.ClerkSecretKey)
		logger.Printf("clerk: secret key configured")
	} else {
		logger.Printf("clerk: no CLERK_SECRET_KEY, /api/me will reject all requests")
	}

	logger.Printf("config: HTTP_ADDR=%s", cfg.HTTPAddr)
	logger.Printf("config: EDITH_DB_PATH=%s", cfg.EdithDBPath)
	logger.Printf("config: SESSION_DB_PATH=%s", cfg.SessionDBPath)

	if err := openAndMigrate(cfg, logger); err != nil {
		logger.Fatalf("store init failed: %v", err)
	}

	srv := httpserver.New(cfg.HTTPAddr, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Printf("signal received, shutting down")
	case err := <-errCh:
		if err != nil {
			logger.Fatalf("httpserver exited with error: %v", err)
		}
		logger.Printf("httpserver exited cleanly")
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.Printf("shutdown error: %v", err)
	}

	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Printf("httpserver final error: %v", err)
	}
	logger.Printf("bye")
}

// openAndMigrate 打开 EDITH 业务库，跑幂等迁移，确认 SQLite 可用后关闭。
// 阶段 1 用它做启动期自检；阶段 3+ 会把 *store.Store 注入 httpserver / runtime。
func openAndMigrate(cfg *config.Config, logger *log.Logger) error {
	s, err := store.Open(cfg.EdithDBPath)
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Migrate(context.Background()); err != nil {
		return err
	}
	logger.Printf("store: edith.db ready at %s", cfg.EdithDBPath)
	return nil
}