package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound 是所有 Repository "查不到" 的统一定义。
var ErrNotFound = errors.New("store: not found")

type User struct {
	ClerkUserID  string
	GitHubUserID int64
}

// UpsertUser 按 clerk_user_id 主键 upsert。
func (s *Store) UpsertUser(ctx context.Context, u User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (clerk_user_id, github_user_id)
		 VALUES (?, ?)
		 ON CONFLICT(clerk_user_id) DO UPDATE SET github_user_id = excluded.github_user_id`,
		u.ClerkUserID, u.GitHubUserID,
	)
	if err != nil {
		return fmt.Errorf("store: upsert user %s: %w", u.ClerkUserID, err)
	}
	return nil
}

// FindUserByClerkID 查不到返回 ErrNotFound。
func (s *Store) FindUserByClerkID(ctx context.Context, clerkUserID string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT clerk_user_id, github_user_id FROM users WHERE clerk_user_id = ?`,
		clerkUserID,
	).Scan(&u.ClerkUserID, &u.GitHubUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: find user %s: %w", clerkUserID, err)
	}
	return u, nil
}
