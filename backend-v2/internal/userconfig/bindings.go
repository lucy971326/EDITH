package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Bindings 提供外部渠道账号到 Clerk 用户的映射。
type Bindings struct {
	store *bindingStore
}

// Bind 保存一个渠道账号的归属。
func (b *Bindings) Bind(ctx context.Context, binding ChannelBinding) error {
	return b.store.bind(ctx, binding)
}

// ToClerkUserID 将渠道账号转换为 Clerk 用户；未绑定时 found 为 false。
func (b *Bindings) ToClerkUserID(ctx context.Context, channel, externalUserID string) (clerkUserID string, found bool, err error) {
	return b.store.toClerkUserID(ctx, channel, externalUserID)
}

// bindingStore 是 Bindings 的私有持久化细节。
type bindingStore struct {
	db *sql.DB
}

func (s *bindingStore) bind(ctx context.Context, binding ChannelBinding) error {
	binding.Channel = strings.TrimSpace(binding.Channel)
	binding.ExternalUserID = strings.TrimSpace(binding.ExternalUserID)
	binding.ClerkUserID = strings.TrimSpace(binding.ClerkUserID)
	if binding.Channel == "" || binding.ExternalUserID == "" || binding.ClerkUserID == "" {
		return errors.New("channel, external user id, and clerk user id are required")
	}
	if err := ensureUser(ctx, s.db, binding.ClerkUserID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_channel_bindings (channel, external_user_id, clerk_user_id)
		VALUES (?, ?, ?)
		ON CONFLICT(channel, external_user_id) DO UPDATE SET clerk_user_id = excluded.clerk_user_id
	`, binding.Channel, binding.ExternalUserID, binding.ClerkUserID)
	if err != nil {
		return fmt.Errorf("bind channel user: %w", err)
	}
	return nil
}

func (s *bindingStore) toClerkUserID(ctx context.Context, channel, externalUserID string) (string, bool, error) {
	channel = strings.TrimSpace(channel)
	externalUserID = strings.TrimSpace(externalUserID)
	if channel == "" || externalUserID == "" {
		return "", false, errors.New("channel and external user id are required")
	}
	var clerkUserID string
	err := s.db.QueryRowContext(ctx, `
		SELECT clerk_user_id FROM user_channel_bindings
		WHERE channel = ? AND external_user_id = ?
	`, channel, externalUserID).Scan(&clerkUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup channel user: %w", err)
	}
	return clerkUserID, true, nil
}

func ensureUser(ctx context.Context, db *sql.DB, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID); err != nil {
		return fmt.Errorf("ensure user %q: %w", userID, err)
	}
	return nil
}
