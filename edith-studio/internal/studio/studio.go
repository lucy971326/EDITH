// Package studio 提供 EDITH Studio 的本地 HTTP 应用。
package studio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"edith/studio/internal/engine"
)

// Run 启动 Engine 和本地 HTTP 服务，并在进程退出时释放资源。
func Run(ctx context.Context, projectRoot, address string) error {
	engineRuntime, err := engine.Open(projectRoot)
	if err != nil {
		return err
	}
	defer engineRuntime.Close()

	server := &http.Server{
		Addr:              address,
		Handler:           newHandler(ctx, engineRuntime),
		ReadHeaderTimeout: 10 * time.Second,
	}
	shutdownCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-shutdownCh:
		}
	}()
	err = server.ListenAndServe()
	close(shutdownCh)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve Studio HTTP: %w", err)
}
