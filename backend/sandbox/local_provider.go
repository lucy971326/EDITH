package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LocalProvider gives every workspace its own local directory backend.
type LocalProvider struct {
	rootDir         string
	systemSkillsDir string
	userSkillsRoot  string

	mu    sync.RWMutex
	cache map[WorkspaceID]ExecBackend
}

// NewLocalProvider creates a local workspace provider.
// systemSkillsDir is the read-only source behind the Agent-visible skills/system path.
func NewLocalProvider(rootDir, systemSkillsDir string) (*LocalProvider, error) {
	abs, err := filepath.Abs(filepath.Clean(rootDir))
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root %q: %w", rootDir, err)
	}
	if err := os.MkdirAll(abs, defaultCreateDirMode); err != nil {
		return nil, fmt.Errorf("create workspace root %q: %w", abs, err)
	}
	userSkillsRoot := filepath.Join(filepath.Dir(abs), "user-skills")
	if err := os.MkdirAll(userSkillsRoot, defaultCreateDirMode); err != nil {
		return nil, fmt.Errorf("create user skills root %q: %w", userSkillsRoot, err)
	}
	var skillsAbs string
	if systemSkillsDir != "" {
		skillsAbs, err = filepath.Abs(filepath.Clean(systemSkillsDir))
		if err != nil {
			return nil, fmt.Errorf("resolve system skills directory %q: %w", systemSkillsDir, err)
		}
		if info, statErr := os.Stat(skillsAbs); statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("system skills directory %q is not available", skillsAbs)
		}
	}

	return &LocalProvider{
		rootDir:         abs,
		systemSkillsDir: skillsAbs,
		userSkillsRoot:  userSkillsRoot,
		cache:           make(map[WorkspaceID]ExecBackend),
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
	userSkillsDir, err := p.ensureUserSkillsDir(id.UserID)
	if err != nil {
		return nil, err
	}
	backend, err := NewLocalBackend(dir, p.systemSkillsDir, userSkillsDir)
	if err != nil {
		return nil, err
	}

	p.cache[id] = backend
	return backend, nil
}

// LoadUserSkillsOverview reads this user's permanent local Skill index.
func (p *LocalProvider) LoadUserSkillsOverview(_ context.Context, userID string) (string, error) {
	dir, err := p.ensureUserSkillsDir(userID)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(dir, "OVERVIEW.md"))
	if err != nil {
		return "", fmt.Errorf("read local user skills overview: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

func (p *LocalProvider) ensureUserSkillsDir(userID string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user ID is required")
	}
	dir := filepath.Join(p.userSkillsRoot, userSkillsDirName(userID))
	if err := os.MkdirAll(dir, defaultCreateDirMode); err != nil {
		return "", fmt.Errorf("create local user skills directory: %w", err)
	}
	overviewPath := filepath.Join(dir, "OVERVIEW.md")
	if _, err := os.Stat(overviewPath); os.IsNotExist(err) {
		if err := os.WriteFile(overviewPath, nil, defaultCreateFileMode); err != nil {
			return "", fmt.Errorf("initialize local user skills overview: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("access local user skills overview: %w", err)
	}
	return dir, nil
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

func userSkillsDirName(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(sum[:])
}
