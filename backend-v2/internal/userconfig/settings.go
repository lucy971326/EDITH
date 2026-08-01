package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Settings 提供用户 Agent 设置的读写能力。
type Settings struct {
	store *settingsStore
}

// Load 读取用户设置；首次读取会建立空白用户记录。
func (s *Settings) Load(ctx context.Context, userID string) (AgentSettings, error) {
	return s.store.load(ctx, userID)
}

// Save 保存人格、默认模型和时区，不处理供应商密钥。
func (s *Settings) Save(ctx context.Context, userID string, settings AgentSettings) error {
	return s.store.save(ctx, userID, settings)
}

// LoadPersonality 为一次 Agent 运行读取人格提示词。
func (s *Settings) LoadPersonality(ctx context.Context, userID string) (string, error) {
	settings, err := s.Load(ctx, userID)
	return settings.Personality, err
}

// LoadDefaultModelID 读取用户默认模型；空字符串表示尚未选择。
func (s *Settings) LoadDefaultModelID(ctx context.Context, userID string) (string, error) {
	settings, err := s.Load(ctx, userID)
	return settings.DefaultModelID, err
}

// LoadTimezone 读取用户时区；空字符串表示调用方应使用默认时区。
func (s *Settings) LoadTimezone(ctx context.Context, userID string) (string, error) {
	settings, err := s.Load(ctx, userID)
	return settings.Timezone, err
}

// SaveTimezone 仅更新时区，供定时任务等功能调用。
func (s *Settings) SaveTimezone(ctx context.Context, userID, timezone string) error {
	settings, err := s.Load(ctx, userID)
	if err != nil {
		return err
	}
	settings.Timezone = timezone
	return s.Save(ctx, userID, settings)
}

// settingsStore 是 Settings 的私有持久化细节。
type settingsStore struct {
	db *sql.DB
}

func (s *settingsStore) load(ctx context.Context, userID string) (AgentSettings, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AgentSettings{}, errors.New("user id is required")
	}
	if err := ensureUser(ctx, s.db, userID); err != nil {
		return AgentSettings{}, err
	}
	var settings AgentSettings
	err := s.db.QueryRowContext(ctx, `
		SELECT personality, default_model_id, timezone
		FROM user_agents WHERE user_id = ?
	`, userID).Scan(&settings.Personality, &settings.DefaultModelID, &settings.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentSettings{}, nil
	}
	if err != nil {
		return AgentSettings{}, fmt.Errorf("load settings for %q: %w", userID, err)
	}
	return settings, nil
}

func (s *settingsStore) save(ctx context.Context, userID string, settings AgentSettings) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_agents (user_id, personality, default_model_id, timezone)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			personality = excluded.personality,
			default_model_id = excluded.default_model_id,
			timezone = excluded.timezone
	`, userID, strings.TrimSpace(settings.Personality), strings.TrimSpace(settings.DefaultModelID), strings.TrimSpace(settings.Timezone))
	if err != nil {
		return fmt.Errorf("save settings for %q: %w", userID, err)
	}
	return nil
}
