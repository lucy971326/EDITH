package store

import (
	"context"
	"errors"
	"testing"
)

func TestUpsertUser_SuccessAndUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertUser(ctx, User{ClerkUserID: "user_1", GitHubUserID: 1001}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	u, err := s.FindUserByClerkID(ctx, "user_1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if u.GitHubUserID != 1001 {
		t.Fatalf("github_user_id = %d, want 1001", u.GitHubUserID)
	}

	// upsert 走 ON CONFLICT DO UPDATE；GitHub ID 变了应被覆盖
	if err := s.UpsertUser(ctx, User{ClerkUserID: "user_1", GitHubUserID: 1002}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	u, _ = s.FindUserByClerkID(ctx, "user_1")
	if u.GitHubUserID != 1002 {
		t.Fatalf("after update github_user_id = %d, want 1002", u.GitHubUserID)
	}
}

func TestFindUserByClerkID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.FindUserByClerkID(context.Background(), "user_nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpsertUser_GitHubIDUniqueConflict(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertUser(ctx, User{ClerkUserID: "user_a", GitHubUserID: 5000}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 不同 clerk_user_id 相同 github_user_id → UNIQUE(github_user_id) 冲突
	err := s.UpsertUser(ctx, User{ClerkUserID: "user_b", GitHubUserID: 5000})
	if err == nil {
		t.Fatalf("expected UNIQUE conflict on github_user_id, got nil")
	}
}