package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Providers 提供用户模型供应商密钥的服务端读写能力。
type Providers struct {
	store *providerStore
}

// ListStatuses 返回可安全暴露给浏览器的密钥配置状态。
func (p *Providers) ListStatuses(ctx context.Context, userID string) ([]ProviderStatus, error) {
	return p.store.listStatuses(ctx, userID)
}

// Save 保存提交的密钥；APIKey 为 nil 的供应商保持不变。
func (p *Providers) Save(ctx context.Context, userID string, credentials []ProviderCredential) error {
	return p.store.save(ctx, userID, credentials)
}

// LoadAPIKey 为一次 Agent 运行读取一个供应商的密钥。
func (p *Providers) LoadAPIKey(ctx context.Context, userID, providerID string) (string, error) {
	return p.store.loadAPIKey(ctx, userID, providerID)
}

// providerStore 是 Providers 的私有持久化细节。
type providerStore struct {
	db *sql.DB
}

func (s *providerStore) listStatuses(ctx context.Context, userID string) ([]ProviderStatus, error) {
	if err := ensureUser(ctx, s.db, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, api_key <> '' FROM user_providers
		WHERE user_id = ? ORDER BY provider_id
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list provider statuses for %q: %w", userID, err)
	}
	defer rows.Close()

	statuses := []ProviderStatus{}
	for rows.Next() {
		var status ProviderStatus
		if err := rows.Scan(&status.ProviderID, &status.HasAPIKey); err != nil {
			return nil, fmt.Errorf("scan provider status: %w", err)
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider statuses: %w", err)
	}
	return statuses, nil
}

func (s *providerStore) save(ctx context.Context, userID string, credentials []ProviderCredential) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start provider save: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID); err != nil {
		return fmt.Errorf("ensure user %q: %w", userID, err)
	}
	for _, credential := range credentials {
		providerID := strings.TrimSpace(credential.ProviderID)
		if providerID == "" {
			return errors.New("provider id is required")
		}
		if credential.APIKey == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_providers (user_id, provider_id, api_key)
			VALUES (?, ?, ?)
			ON CONFLICT(user_id, provider_id) DO UPDATE SET api_key = excluded.api_key
		`, userID, providerID, strings.TrimSpace(*credential.APIKey)); err != nil {
			return fmt.Errorf("save provider %q: %w", providerID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider save: %w", err)
	}
	return nil
}

func (s *providerStore) loadAPIKey(ctx context.Context, userID, providerID string) (string, error) {
	var apiKey string
	err := s.db.QueryRowContext(ctx, `
		SELECT api_key FROM user_providers WHERE user_id = ? AND provider_id = ?
	`, strings.TrimSpace(userID), strings.TrimSpace(providerID)).Scan(&apiKey)
	if err != nil {
		return "", fmt.Errorf("load provider key %q/%q: %w", userID, providerID, err)
	}
	return apiKey, nil
}
