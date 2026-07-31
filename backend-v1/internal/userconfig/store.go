// Package userconfig stores EDITH's user-level runtime configuration.
package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Store uses EDITH's long-lived SQLite connection.
type Store struct {
	db *sql.DB
}

// Open creates EDITH's current tables on the caller-owned SQLite connection.
func Open(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("user config database is required")
	}
	store := &Store{db: db}
	if err := store.createTables(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

// LoadSettings creates a user record on first visit and returns only safe
// credential status for every configured provider.
func (s *Store) LoadSettings(ctx context.Context, userID string) (Settings, []ProviderStatus, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return Settings{}, nil, err
	}

	settings := Settings{}
	err := s.db.QueryRowContext(ctx, `SELECT personality, default_model_id FROM user_agents WHERE user_id = ?`, userID).Scan(&settings.Personality, &settings.DefaultModelID)
	if errors.Is(err, sql.ErrNoRows) {
		settings.Personality = ""
		settings.DefaultModelID = ""
	} else if err != nil {
		return Settings{}, nil, fmt.Errorf("load user agent settings %q: %w", userID, err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT provider_id, api_key <> '' FROM user_providers WHERE user_id = ?`, userID)
	if err != nil {
		return Settings{}, nil, fmt.Errorf("load provider settings %q: %w", userID, err)
	}
	defer rows.Close()

	// Keep browser JSON stable: no configured providers is [], never null.
	statuses := []ProviderStatus{}
	for rows.Next() {
		var status ProviderStatus
		if err := rows.Scan(&status.ProviderID, &status.HasAPIKey); err != nil {
			return Settings{}, nil, fmt.Errorf("scan provider settings %q: %w", userID, err)
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return Settings{}, nil, fmt.Errorf("iterate provider settings %q: %w", userID, err)
	}
	return settings, statuses, nil
}

// SaveSettings writes personality and any submitted provider credentials. A
// nil API key leaves that provider untouched, so secrets never need to return
// to the browser.
func (s *Store) SaveSettings(ctx context.Context, userID string, settings Settings) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start save user settings %q: %w", userID, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID); err != nil {
		return fmt.Errorf("ensure user %q: %w", userID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_agents (user_id, personality, default_model_id)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			personality = excluded.personality,
			default_model_id = excluded.default_model_id
	`, userID, settings.Personality, settings.DefaultModelID); err != nil {
		return fmt.Errorf("save user agent config %q: %w", userID, err)
	}

	for _, provider := range settings.Providers {
		if provider.APIKey == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_providers (user_id, provider_id, api_key)
			VALUES (?, ?, ?)
			ON CONFLICT(user_id, provider_id) DO UPDATE SET api_key = excluded.api_key
		`, userID, provider.ProviderID, *provider.APIKey); err != nil {
			return fmt.Errorf("save provider config %q/%q: %w", userID, provider.ProviderID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save user settings %q: %w", userID, err)
	}
	return nil
}

// LoadProviderAPIKey obtains one credential only for server-side RunOptions.
func (s *Store) LoadProviderAPIKey(ctx context.Context, userID, providerID string) (string, error) {
	var apiKey string
	err := s.db.QueryRowContext(ctx, `
		SELECT api_key FROM user_providers WHERE user_id = ? AND provider_id = ?
	`, userID, providerID).Scan(&apiKey)
	if err != nil {
		return "", fmt.Errorf("load provider API key %q/%q: %w", userID, providerID, err)
	}
	return apiKey, nil
}

// LoadPersonality obtains the non-secret instruction fragment for one run.
func (s *Store) LoadPersonality(ctx context.Context, userID string) (string, error) {
	var personality string
	err := s.db.QueryRowContext(ctx, `SELECT personality FROM user_agents WHERE user_id = ?`, userID).Scan(&personality)
	if err != nil {
		return "", fmt.Errorf("load user personality %q: %w", userID, err)
	}
	return personality, nil
}

// LoadDefaultModelID 读取用户明确保存的默认模型。
// 输出为空表示用户尚未选择，调用方应回退到当前系统默认模型。
func (s *Store) LoadDefaultModelID(ctx context.Context, userID string) (string, error) {
	var modelID string
	err := s.db.QueryRowContext(ctx, `SELECT default_model_id FROM user_agents WHERE user_id = ?`, userID).Scan(&modelID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load default model %q: %w", userID, err)
	}
	return strings.TrimSpace(modelID), nil
}

// BindChannelUser 保存渠道账号到 Clerk 用户的绑定。
// 输入：渠道名、平台用户标识和 Clerk 用户 ID。
// 输出：同一渠道账号以后会解析为该 Clerk 用户。
func (s *Store) BindChannelUser(ctx context.Context, binding ChannelBinding) error {
	binding.Channel = strings.TrimSpace(binding.Channel)
	binding.ExternalUserID = strings.TrimSpace(binding.ExternalUserID)
	binding.UserID = strings.TrimSpace(binding.UserID)
	if binding.Channel == "" || binding.ExternalUserID == "" || binding.UserID == "" {
		return errors.New("channel, external user id, and user id are required")
	}
	if err := s.ensureUser(ctx, binding.UserID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_channel_bindings (channel, external_user_id, user_id)
		VALUES (?, ?, ?)
		ON CONFLICT(channel, external_user_id) DO UPDATE SET user_id = excluded.user_id
	`, binding.Channel, binding.ExternalUserID, binding.UserID)
	if err != nil {
		return fmt.Errorf("bind channel user %q/%q: %w", binding.Channel, binding.ExternalUserID, err)
	}
	return nil
}

// LookupChannelUser 将渠道账号解析为已绑定的 Clerk 用户。
// 输出：没有绑定时返回 found=false，不把“未绑定”伪装成数据库错误。
func (s *Store) LookupChannelUser(ctx context.Context, channel, externalUserID string) (userID string, found bool, err error) {
	channel = strings.TrimSpace(channel)
	externalUserID = strings.TrimSpace(externalUserID)
	if channel == "" || externalUserID == "" {
		return "", false, errors.New("channel and external user id are required")
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT user_id FROM user_channel_bindings WHERE channel = ? AND external_user_id = ?
	`, channel, externalUserID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup channel user %q/%q: %w", channel, externalUserID, err)
	}
	return userID, true, nil
}

func (s *Store) ensureUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID)
	if err != nil {
		return fmt.Errorf("ensure user %q: %w", userID, err)
	}
	return nil
}

func (s *Store) createTables(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_agents (
			user_id TEXT PRIMARY KEY,
			personality TEXT NOT NULL DEFAULT '',
			default_model_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS user_providers (
			user_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			api_key TEXT NOT NULL,
			PRIMARY KEY (user_id, provider_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_mcp_servers (
			server_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			transport TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_mcp_headers (
			server_id TEXT NOT NULL,
			header_name TEXT NOT NULL,
			header_value TEXT NOT NULL,
			PRIMARY KEY (server_id, header_name)
		)`,
		`CREATE TABLE IF NOT EXISTS user_channel_bindings (
			channel TEXT NOT NULL,
			external_user_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			PRIMARY KEY (channel, external_user_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create user config table: %w", err)
		}
	}
	if err := s.ensureDefaultModelColumn(ctx); err != nil {
		return err
	}
	return nil
}

// ensureDefaultModelColumn 为已有 SQLite 数据库补齐新增字段。
func (s *Store) ensureDefaultModelColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(user_agents)`)
	if err != nil {
		return fmt.Errorf("inspect user agent columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			columnID int
			name     string
			typeName string
			notNull  int
			defaultV any
			primary  int
		)
		if err := rows.Scan(&columnID, &name, &typeName, &notNull, &defaultV, &primary); err != nil {
			return fmt.Errorf("scan user agent column: %w", err)
		}
		if name == "default_model_id" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate user agent columns: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE user_agents ADD COLUMN default_model_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add default model column: %w", err)
	}
	return nil
}
