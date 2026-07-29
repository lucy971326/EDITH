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
	settings := Settings{Personality: "简洁。", Providers: []ProviderCredential{{ProviderID: "deepseek", APIKey: &apiKey}}}
	if err := store.SaveSettings(context.Background(), "alice", settings); err != nil {
		t.Fatal(err)
	}
	loaded, statuses, err := store.LoadSettings(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Personality != "简洁。" || len(statuses) != 1 || !statuses[0].HasAPIKey {
		t.Fatalf("settings = %#v, statuses = %#v", loaded, statuses)
	}
	got, err := store.LoadProviderAPIKey(context.Background(), "alice", "deepseek")
	if err != nil || got != apiKey {
		t.Fatalf("API key = %q, err = %v", got, err)
	}
}
