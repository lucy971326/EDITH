// Package session 管理本地 SQLite 会话存储。
package session

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

// Module 是本地 SQLite SessionService 的长期所有者。
type Module struct {
	// service 是提供给 Engine 和后续会话界面的框架会话能力。
	service frameworksession.Service
}

// Create 创建用户目录中的 SQLite 会话 Module。
func Create() (*Module, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find user home directory: %w", err)
	}
	dataDir := filepath.Join(homeDir, ".edith")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dataDir, err)
	}
	databasePath := filepath.Join(dataDir, "sessions.db")
	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open session database: %w", err)
	}
	service, err := sqlite.NewService(database)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create session service: %w", err)
	}
	return &Module{service: service}, nil
}

// Service 返回供 Agent Runner 保存会话的框架 SessionService。
func (m *Module) Service() frameworksession.Service {
	return m.service
}

// Close 关闭 Module 持有的 SQLite SessionService。
func (m *Module) Close() error {
	return m.service.Close()
}
