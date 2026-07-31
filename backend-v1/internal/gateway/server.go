package gateway

import (
	"database/sql"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/onlyrun"
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Gateway 是渠道调用 OnlyRun 的统一门面。
// 它不持有执行细节，也不理解 HTTP、SSE 或 IM 平台协议。
type Gateway struct {
	users   *userconfig.Store
	onlyRun *onlyrun.OnlyRun
}

func New(
	runner runner.ManagedRunner,
	users *userconfig.Store,
	images *images.Service,
	usageDB *sql.DB,
) (*Gateway, error) {
	executor, err := onlyrun.New(runner, users, images, usageDB)
	if err != nil {
		return nil, err
	}
	return &Gateway{users: users, onlyRun: executor}, nil
}
