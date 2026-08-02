package volume

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreSaveAndLoad(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:volume-store-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx := context.Background()
	if err := createSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := &store{db: db}
	want := record{UserID: "user-1", ID: "vol-1", Name: "edith-user-volume-1", Token: "secret"}
	if err := s.save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.load(ctx, want.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("loaded record = %+v, want %+v", got, want)
	}
}

func TestNewRequiresDatabase(t *testing.T) {
	if _, err := New(Dependencies{}); err == nil {
		t.Fatal("New accepted nil database")
	}
}

func TestNewResolvesE2BConfig(t *testing.T) {
	t.Setenv("E2B_API_KEY", "volume-test-key")
	db, err := sql.Open("sqlite3", "file:volume-config-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	module, err := New(Dependencies{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	if module.Volumes.config.APIKey != "volume-test-key" {
		t.Fatalf("resolved API key = %q", module.Volumes.config.APIKey)
	}
}

func TestMountForUserRequiresUserID(t *testing.T) {
	service := &Service{store: &store{}}
	if _, err := service.MountForUser(context.Background(), " "); err == nil {
		t.Fatal("MountForUser accepted an empty user ID")
	}
}
