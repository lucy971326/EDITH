package store

import (
	"context"
	"path/filepath"
	"testing"
)

// newTestStore 给单测用的内存 SQLite 工厂。
// 每个测试一个独立 t.TempDir() 路径，避免相互污染。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "edith.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}