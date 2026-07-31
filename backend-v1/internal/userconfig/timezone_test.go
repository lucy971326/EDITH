package userconfig

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestStoreSavesAndLoadsTimezone(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTimezone(context.Background(), "alice", "Asia/Tokyo"); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadTimezone(context.Background(), "alice")
	if err != nil || got != "Asia/Tokyo" {
		t.Fatalf("timezone = %q, err = %v", got, err)
	}
	// 未设置的用户返回空，由调用方兜底。
	empty, err := store.LoadTimezone(context.Background(), "bob")
	if err != nil || empty != "" {
		t.Fatalf("empty timezone = %q, err = %v", empty, err)
	}
}

func TestStoreSaveSettingsIncludesTimezone(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSettings(context.Background(), "alice", Settings{Personality: "简洁。", DefaultModelID: "deepseek-v3", Timezone: "Asia/Shanghai"}); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := store.LoadSettings(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone = %q, want Asia/Shanghai", loaded.Timezone)
	}
}
