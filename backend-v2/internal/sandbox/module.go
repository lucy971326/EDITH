// Package sandbox 提供按用户和会话隔离的 E2B 工作区及其 Agent 工具。
package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"edith/backend-v2/internal/volume"
	"github.com/eric642/e2b-go-sdk"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const connectTimeout = 10 * time.Minute

// Dependencies 是创建 Sandbox 模块需要的进程级资源。
type Dependencies struct {
	DB       *sql.DB
	Template string
	Volumes  *volume.Service
}

// Module 是 Sandbox 功能的公开入口；Tools 供 tools 聚合模块收集。
type Module struct {
	Tools tool.ToolSet
}

// New 创建工作区映射表和 E2B 客户端；不会创建任何用户 Sandbox。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("sandbox requires a database")
	}
	template := strings.TrimSpace(deps.Template)
	if template == "" {
		return nil, errors.New("sandbox requires an E2B template")
	}
	if deps.Volumes == nil {
		return nil, errors.New("sandbox requires a volume service")
	}
	if err := createSchema(context.Background(), deps.DB); err != nil {
		return nil, err
	}
	client, err := e2b.NewClient(e2b.Config{})
	if err != nil {
		return nil, fmt.Errorf("create E2B client: %w", err)
	}
	workspaces := &service{db: deps.DB, client: client, template: template, volumes: deps.Volumes}
	return &Module{Tools: &toolSet{workspaces: workspaces}}, nil
}
