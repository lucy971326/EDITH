// Package session 管理本地 SQLite 会话存储。
package session

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

// Open 创建持久化到用户目录的 SessionService。
func Open() (session.Service, error) {
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
	return service, nil
}
