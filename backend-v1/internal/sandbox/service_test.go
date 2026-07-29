package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSandboxMappingTable(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "edith.db")
	service, err := Open(databasePath, "edith-test")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer service.Close()

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
