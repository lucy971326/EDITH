// Package studio 提供 EDITH Studio 的本地 HTTP 应用。
package studio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"edith/studio/internal/workspace"
)

// Start 创建 Workspace 并启动本地 HTTP 服务；进程退出时关闭 Workspace。
func Start(ctx context.Context, projectRoot, address string) (returnErr error) {
	workspaceRuntime, err := workspace.Create(workspace.Dependencies{ProjectRoot: projectRoot})
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, workspaceRuntime.Close())
	}()

	server := &http.Server{
		Addr:              address,
		Handler:           newHandler(ctx, workspaceRuntime),
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
