package store

import (
	"context"
	"errors"
	"testing"
)

func TestSaveInstallation_SuccessAndFind(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 必须先有 users 行，否则 FK 报错
	if err := s.UpsertUser(ctx, User{ClerkUserID: "user_1", GitHubUserID: 1001}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := s.SaveInstallation(ctx, Installation{InstallationID: 7777, ClerkUserID: "user_1"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	ins, err := s.FindInstallationByID(ctx, 7777)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if ins.ClerkUserID != "user_1" {
		t.Fatalf("clerk_user_id = %q, want user_1", ins.ClerkUserID)
	}
}

func TestFindInstallationByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.FindInstallationByID(context.Background(), 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSaveInstallation_OneUserOneInstallation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.UpsertUser(ctx, User{ClerkUserID: "user_1", GitHubUserID: 1001})
	_ = s.UpsertUser(ctx, User{ClerkUserID: "user_2", GitHubUserID: 1002})

	if err := s.SaveInstallation(ctx, Installation{InstallationID: 1, ClerkUserID: "user_1"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 同一 clerk_user_id 再绑一次会被 UNIQUE 拒绝（一人一 Installation）
	err := s.SaveInstallation(ctx, Installation{InstallationID: 2, ClerkUserID: "user_1"})
	if err == nil {
		t.Fatalf("expected UNIQUE conflict on clerk_user_id, got nil")
	}
}