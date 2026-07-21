package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LocalProvider gives every workspace its own local directory backend.
type LocalProvider struct {
	rootDir string

	mu    sync.RWMutex
	cache map[WorkspaceID]ExecBackend
}

// NewLocalProvider creates a backend provider rooted at rootDir.
func NewLocalProvider(rootDir string) (*LocalProvider, error) {
	abs, err := filepath.Abs(filepath.Clean(rootDir))
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root %q: %w", rootDir, err)
	}
	if err := os.MkdirAll(abs, defaultCreateDirMode); err != nil {
		return nil, fmt.Errorf("create workspace root %q: %w", abs, err)
	}

	return &LocalProvider{
		rootDir: abs,
		cache:   make(map[WorkspaceID]ExecBackend),
	}, nil
}

// GetBackend returns a local backend isolated to the given workspace.
func (p *LocalProvider) GetBackend(_ context.Context, id WorkspaceID) (ExecBackend, error) {
	if err := id.validate(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	if backend, ok := p.cache[id]; ok {
		p.mu.RUnlock()
		return backend, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if backend, ok := p.cache[id]; ok {
		return backend, nil
	}

	dir := filepath.Join(p.rootDir, workspaceDirName(id))
	if err := os.MkdirAll(dir, defaultCreateDirMode); err != nil {
		return nil, fmt.Errorf("create local workspace: %w", err)
	}
	backend, err := NewLocalBackend(dir)
	if err != nil {
		return nil, err
	}

	p.cache[id] = backend
	return backend, nil
}

// Close releases all cached local backends.
func (p *LocalProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for id, backend := range p.cache {
		if err := backend.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.cache, id)
	}
	return firstErr
}

func workspaceDirName(id WorkspaceID) string {
	sum := sha256.Sum256([]byte(id.UserID + "\x00" + id.SessionID))
	return hex.EncodeToString(sum[:])
}
