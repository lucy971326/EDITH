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
// SandboxManager — manages the mapping of userID → E2B sandbox.
// ============================================================================

// NewSandboxManager creates a manager backed by the given SQLite DB.
// The DB must already be opened with the sqlite3 driver.
func NewSandboxManager(db *sql.DB, opts SandboxManagerOptions) (*SandboxManager, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sandboxes (
		user_id    TEXT PRIMARY KEY,
		sandbox_id TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create sandboxes table: %w", err)
	}

	cfg := e2b.Config{
		APIKey: opts.APIKey,
		Domain: opts.Domain,
	}
	client, err := e2b.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create e2b client: %w", err)
	}

	return &SandboxManager{
		db:     db,
		client: client,
		opts:   opts,
		cache:  make(map[string]*e2b.Sandbox),
	}, nil
}

// SandboxManagerOptions configures sandbox creation.
type SandboxManagerOptions struct {
	APIKey   string        // E2B API key, also read from E2B_API_KEY env
	Domain   string        // E2B domain, default "e2b.app"
	Template string        // sandbox template, default "base"
	Timeout  time.Duration // sandbox timeout, default 10 min
}

func (o SandboxManagerOptions) template() string {
	if o.Template == "" {
		return "base"
	}
	return o.Template
}

func (o SandboxManagerOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 10 * time.Minute
	}
	return o.Timeout
}

type SandboxManager struct {
	db     *sql.DB               // sqlite
	client *e2b.Client           // e2b控制client
	opts   SandboxManagerOptions // default参数

	mu    sync.RWMutex            // 读写锁
	cache map[string]*e2b.Sandbox // userID → sandbox
}

// GetSandbox returns the sandbox for userID, creating one if needed.
// Uses auto-pause + auto-resume for persistent, zero-maintenance sandboxes.
func (m *SandboxManager) GetSandbox(ctx context.Context, userID string) (*e2b.Sandbox, error) {
	// Fast path: already loaded in this process.
	m.mu.RLock()
	if sbx, ok := m.cache[userID]; ok {
		m.mu.RUnlock()
		return sbx, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if sbx, ok := m.cache[userID]; ok {
		return sbx, nil
	}

	// Look up the sandbox ID from SQLite.
	var sandboxID string
	err := m.db.QueryRowContext(ctx,
		"SELECT sandbox_id FROM sandboxes WHERE user_id = ?", userID,
	).Scan(&sandboxID)

	if err == nil {
		// Existing sandbox — reconnect (auto-resume if paused).
		sbx, connectErr := m.client.Connect(ctx, sandboxID, e2b.ConnectOptions{
			Timeout: m.opts.timeout(),
		})
		if connectErr != nil {
			return nil, fmt.Errorf("reconnect sandbox %s for user %s: %w", sandboxID, userID, connectErr)
		}
		m.cache[userID] = sbx
		return sbx, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query sandbox for user %s: %w", userID, err)
	}

	// No existing sandbox — create a new one.
	sbx, createErr := m.client.Create(ctx, e2b.CreateOptions{
		Template: m.opts.template(),
		Timeout:  m.opts.timeout(),
		Lifecycle: &e2b.LifecycleOptions{
			OnTimeout:  "pause",
			AutoResume: true,
		},
	})
	if createErr != nil {
		return nil, fmt.Errorf("create sandbox for user %s: %w", userID, createErr)
	}

	// Persist the mapping.
	if _, err := m.db.ExecContext(ctx,
		"INSERT INTO sandboxes (user_id, sandbox_id, created_at) VALUES (?, ?, ?)",
		userID, sbx.ID, time.Now().Unix(),
	); err != nil {
		// Sandbox created but mapping not saved — still return the sandbox.
		// Next time GetSandbox is called, it'll create a new one.
		return nil, fmt.Errorf("save sandbox mapping for user %s: %w", userID, err)
	}

	m.cache[userID] = sbx
	return sbx, nil
}
