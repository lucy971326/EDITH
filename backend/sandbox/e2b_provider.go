package sandbox

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/eric642/e2b-go-sdk"
	_ "github.com/mattn/go-sqlite3"
)

// ============================================================================
// E2BProvider — provides one E2B backend per workspace.
// ============================================================================

// NewE2BProvider creates a provider backed by the given SQLite DB.
// The DB must already be opened with the sqlite3 driver.
func NewE2BProvider(db *sql.DB, opts E2BProviderOptions) (*E2BProvider, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sandbox_workspaces (
		user_id    TEXT NOT NULL,
		session_id TEXT NOT NULL,
		sandbox_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		PRIMARY KEY (user_id, session_id)
	)`); err != nil {
		return nil, fmt.Errorf("create sandbox_workspaces table: %w", err)
	}
	if err := initUserSkillVolumesTable(db); err != nil {
		return nil, err
	}

	cfg := e2b.Config{
		APIKey: opts.APIKey,
		Domain: opts.Domain,
	}
	client, err := e2b.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create e2b client: %w", err)
	}

	return &E2BProvider{
		db:     db,
		client: client,
		opts:   opts,
		cache:  make(map[WorkspaceID]ExecBackend),
	}, nil
}

// E2BProviderOptions configures sandbox creation.
type E2BProviderOptions struct {
	APIKey   string        // E2B API key, also read from E2B_API_KEY env
	Domain   string        // E2B domain, default "e2b.app"
	Template string        // sandbox template, default "base"
	Timeout  time.Duration // sandbox timeout, default 10 min
}

func (o E2BProviderOptions) template() string {
	if o.Template == "" {
		return "base"
	}
	return o.Template
}

func (o E2BProviderOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 10 * time.Minute
	}
	return o.Timeout
}

type E2BProvider struct {
	db     *sql.DB            // workspace mapping persistence
	client *e2b.Client        // E2B control client
	opts   E2BProviderOptions // sandbox defaults

	mu    sync.RWMutex
	cache map[WorkspaceID]ExecBackend

	userSkillsMu sync.Mutex // 只保护同一时刻首次创建用户 Volume 的事务
}

// GetBackend returns the backend for a workspace, creating one if needed.
// Uses auto-pause + auto-resume for persistent, zero-maintenance sandboxes.
func (p *E2BProvider) GetBackend(ctx context.Context, id WorkspaceID) (ExecBackend, error) {
	if err := id.validate(); err != nil {
		return nil, err
	}

	// Fast path: already loaded in this process.
	p.mu.RLock()
	if backend, ok := p.cache[id]; ok {
		p.mu.RUnlock()
		return backend, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if backend, ok := p.cache[id]; ok {
		return backend, nil
	}

	// Look up the sandbox ID from SQLite.
	var sandboxID string
	err := p.db.QueryRowContext(ctx,
		"SELECT sandbox_id FROM sandbox_workspaces WHERE user_id = ? AND session_id = ?",
		id.UserID, id.SessionID,
	).Scan(&sandboxID)

	if err == nil {
		// Existing sandbox — reconnect (auto-resume if paused).
		sbx, connectErr := p.client.Connect(ctx, sandboxID, e2b.ConnectOptions{
			Timeout: p.opts.timeout(),
		})
		if connectErr != nil {
			return nil, fmt.Errorf("reconnect sandbox %s for workspace %s/%s: %w", sandboxID, id.UserID, id.SessionID, connectErr)
		}
		backend := NewE2BBackend(sbx)
		p.cache[id] = backend
		return backend, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query sandbox for workspace %s/%s: %w", id.UserID, id.SessionID, err)
	}

	// No existing sandbox — create a new one.
	userVolume, err := p.ensureUserSkillVolume(ctx, id.UserID)
	if err != nil {
		return nil, fmt.Errorf("ensure user skill volume for %s: %w", id.UserID, err)
	}

	sbx, createErr := p.client.Create(ctx, e2b.CreateOptions{
		Template: p.opts.template(),
		Timeout:  p.opts.timeout(),
		VolumeMounts: []e2b.VolumeMount{
			{
				Name: userVolume.Name,
				Path: "/home/user/skills/user",
			},
		},
		Lifecycle: &e2b.LifecycleOptions{
			OnTimeout:  "pause",
			AutoResume: true,
		},
	})
	if createErr != nil {
		return nil, fmt.Errorf("create sandbox for workspace %s/%s: %w", id.UserID, id.SessionID, createErr)
	}

	// Persist the mapping.
	if _, err := p.db.ExecContext(ctx,
		"INSERT INTO sandbox_workspaces (user_id, session_id, sandbox_id, created_at) VALUES (?, ?, ?, ?)",
		id.UserID, id.SessionID, sbx.ID, time.Now().Unix(),
	); err != nil {
		// The sandbox exists, but without a durable mapping it cannot be safely reused.
		// A later GetBackend call may create a replacement sandbox.
		return nil, fmt.Errorf("save sandbox mapping for workspace %s/%s: %w", id.UserID, id.SessionID, err)
	}

	backend := NewE2BBackend(sbx)
	p.cache[id] = backend
	return backend, nil
}

// Close releases provider-owned process resources.
// E2B sandboxes remain persistent and are paused by their lifecycle policy.
func (p *E2BProvider) Close() error { return nil }
