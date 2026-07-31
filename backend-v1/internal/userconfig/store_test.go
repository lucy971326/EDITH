package userconfig

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestStoreKeepsProviderAPIKeyServerSide(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	apiKey := "alice-api-key"
	settings := Settings{Personality: "简洁。", DefaultModelID: "deepseek-v3", Providers: []ProviderCredential{{ProviderID: "deepseek", APIKey: &apiKey}}}
	if err := store.SaveSettings(context.Background(), "alice", settings); err != nil {
		t.Fatal(err)
	}
	loaded, statuses, err := store.LoadSettings(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Personality != "简洁。" || loaded.DefaultModelID != "deepseek-v3" || len(statuses) != 1 || !statuses[0].HasAPIKey {
		t.Fatalf("settings = %#v, statuses = %#v", loaded, statuses)
	}
	got, err := store.LoadProviderAPIKey(context.Background(), "alice", "deepseek")
	if err != nil || got != apiKey {
		t.Fatalf("API key = %q, err = %v", got, err)
	}
}

func TestStoreBindsChannelUser(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindChannelUser(context.Background(), ChannelBinding{Channel: "feishu", ExternalUserID: "ou_123", UserID: "clerk_123"}); err != nil {
		t.Fatal(err)
	}
	userID, found, err := store.LookupChannelUser(context.Background(), "feishu", "ou_123")
	if err != nil || !found || userID != "clerk_123" {
		t.Fatalf("userID = %q, found = %t, err = %v", userID, found, err)
	}
}

func TestOpenMigratesDefaultModelColumn(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE user_agents (user_id TEXT PRIMARY KEY, personality TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO user_agents (user_id, personality) VALUES ('alice', '简洁。')`); err != nil {
		t.Fatal(err)
	}
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	settings, _, err := store.LoadSettings(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultModelID != "" {
		t.Fatalf("default model = %q, want empty fallback value", settings.DefaultModelID)
	}
}
