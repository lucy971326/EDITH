// Package userconfig stores EDITH's user-level runtime configuration.
package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	err := s.db.QueryRowContext(ctx, `SELECT personality FROM user_agents WHERE user_id = ?`, userID).Scan(&settings.Personality)
	if errors.Is(err, sql.ErrNoRows) {
		settings.Personality = ""
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
		INSERT INTO user_agents (user_id, personality)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET personality = excluded.personality
	`, userID, settings.Personality); err != nil {
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
			personality TEXT NOT NULL DEFAULT ''
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
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create user config table: %w", err)
		}
	}
	return nil
}
