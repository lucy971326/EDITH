package store

import (
	"context"
	"testing"
)

func TestSaveAndFindSandbox(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	key := SessionKey{AppName: "edith", UserID: "user_1", SessionID: "github:foo/bar#1"}

	// 第一次：找不到
	id, found, err := s.FindSandboxID(ctx, key)
	if err != nil || found {
		t.Fatalf("first find: id=%q found=%v err=%v, want (\"\", false, nil)", id, found, err)
	}

	// 保存
	if err := s.SaveSandbox(ctx, key, "sbx_abc"); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 再查：找到
	id, found, err = s.FindSandboxID(ctx, key)
	if err != nil || !found || id != "sbx_abc" {
		t.Fatalf("second find: id=%q found=%v err=%v, want (\"sbx_abc\", true, nil)", id, found, err)
	}

	// 覆盖保存（同一 Session 换 Sandbox）
	if err := s.SaveSandbox(ctx, key, "sbx_def"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	id, _, _ = s.FindSandboxID(ctx, key)
	if id != "sbx_def" {
		t.Fatalf("after overwrite id = %q, want sbx_def", id)
	}
}

func TestSaveSandbox_SandboxIDUnique(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	k1 := SessionKey{AppName: "edith", UserID: "user_1", SessionID: "sess_a"}
	k2 := SessionKey{AppName: "edith", UserID: "user_1", SessionID: "sess_b"}

	if err := s.SaveSandbox(ctx, k1, "sbx_same"); err != nil {
		t.Fatalf("save k1: %v", err)
	}
	// 同一 sandbox_id 绑到不同 Session 应被 UNIQUE 拒绝
	err := s.SaveSandbox(ctx, k2, "sbx_same")
	if err == nil {
		t.Fatalf("expected UNIQUE conflict on sandbox_id, got nil")
	}
}