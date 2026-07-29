package sandbox

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSandboxMappingTable(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "edith.db")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := Open(db, "edith-test")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	_, err = service.db.ExecContext(context.Background(), `
		INSERT INTO user_sandboxes (user_id, session_id, sandbox_id)
		VALUES ('user-1', 'session-1', 'sandbox-1')
	`)
	if err != nil {
		t.Fatalf("insert sandbox mapping: %v", err)
	}

	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("sandbox database was not created: %v", err)
	}
}
