// Package volume 提供按 Clerk 用户隔离的 E2B 持久化 Volume。
package volume

import (
	"context"
	"database/sql"
	"errors"

	"github.com/eric642/e2b-go-sdk"
)

// Dependencies 是创建 Volume 模块需要的进程级资源。
type Dependencies struct {
	DB *sql.DB
}

// Module 是 Volume 功能的公开入口。
// Volumes 供 Sandbox 和未来的 Skills 模块使用。
type Module struct {
	Volumes *Service
}

// New 创建 Volume 映射表和服务；不会立即创建任何远端 Volume。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("volume requires a database")
	}
	if err := createSchema(context.Background(), deps.DB); err != nil {
		return nil, err
	}
	return &Module{Volumes: &Service{store: &store{db: deps.DB}, config: e2b.Config{}.Resolve()}}, nil
}
