// Package session 管理本地 SQLite 会话存储。
package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"
)

// SummaryModelResolver 根据一次请求的 context 返回本次摘要所用的模型。
type SummaryModelResolver func(context.Context) (model.Model, error)

// Module 是本地 SQLite SessionService 的长期所有者。
type Module struct {
	// service 是提供给 Engine 和后续会话界面的框架会话能力。
	service frameworksession.Service
}

// Create 创建用户目录中的 SQLite 会话 Module，并接入动态摘要器。
func Create(resolveSummaryModel SummaryModelResolver) (*Module, error) {
	if resolveSummaryModel == nil {
		return nil, errors.New("summary model resolver is required")
	}
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
	dynamicSummarizer := summary.NewDynamicSummarizer(func(ctx context.Context, _ *frameworksession.Session) (summary.SessionSummarizer, error) {
		summaryModel, err := resolveSummaryModel(ctx)
		if err != nil {
			return nil, err
		}
		if summaryModel == nil {
			return nil, errors.New("summary model resolver returned nil")
		}
		return summary.NewSummarizer(
			summaryModel,
			summary.WithContextThreshold(summary.WithContextThresholdRatio(0.8)),
		), nil
	})
	service, err := sqlite.NewService(database, sqlite.WithSummarizer(dynamicSummarizer))
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
