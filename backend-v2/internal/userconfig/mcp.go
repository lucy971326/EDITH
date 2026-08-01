package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrInvalidMCPServer 表示 MCP 输入不符合安全或格式要求。
	ErrInvalidMCPServer = errors.New("invalid MCP server")
	// ErrMCPServerNotFound 表示该用户没有目标 MCP 服务。
	ErrMCPServerNotFound = errors.New("MCP server not found")
)

// MCP 提供 MCP 服务配置与一次运行所需的已启用配置。
type MCP struct {
	store *mcpStore
}

// List 返回用户全部 MCP 服务，Header 值只留在服务端返回值中。
func (m *MCP) List(ctx context.Context, userID string) ([]MCPServer, error) {
	return m.store.list(ctx, userID, false)
}

// LoadEnabled 返回一次 Agent 运行可连接的 MCP 服务。
func (m *MCP) LoadEnabled(ctx context.Context, userID string) ([]MCPServer, error) {
	return m.store.list(ctx, userID, true)
}

// Create 创建一个属于 userID 的 MCP 服务。
func (m *MCP) Create(ctx context.Context, userID string, input MCPServerInput) (MCPServer, error) {
	return m.store.create(ctx, userID, input)
}

// Update 修改一个属于 userID 的 MCP 服务。
func (m *MCP) Update(ctx context.Context, userID, serverID string, input MCPServerInput) (MCPServer, error) {
	return m.store.update(ctx, userID, serverID, input)
}

// Delete 删除一个属于 userID 的 MCP 服务。
func (m *MCP) Delete(ctx context.Context, userID, serverID string) error {
	return m.store.delete(ctx, userID, serverID)
}

// mcpStore 是 MCP 的私有持久化细节。
type mcpStore struct {
	db *sql.DB
}

func (s *mcpStore) list(ctx context.Context, userID string, enabledOnly bool) ([]MCPServer, error) {
	if err := ensureUser(ctx, s.db, userID); err != nil {
		return nil, err
	}
	query := `SELECT server_id, name, url, transport, enabled FROM user_mcp_servers WHERE user_id = ?`
	if enabledOnly {
		query += ` AND enabled = 1`
	}
	query += ` ORDER BY created_at, server_id`
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}

	servers := []MCPServer{}
	for rows.Next() {
		var server MCPServer
		if err := rows.Scan(&server.ID, &server.Name, &server.URL, &server.Transport, &server.Enabled); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan MCP server: %w", err)
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate MCP servers: %w", err)
	}
	// SQLite 允许单连接部署；先释放列表 rows，再读取每个服务的 Header。
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close MCP server list: %w", err)
	}
	for index := range servers {
		headers, err := s.loadHeaders(ctx, servers[index].ID)
		if err != nil {
			return nil, err
		}
		servers[index].Headers = headers
	}
	return servers, nil
}

func (s *mcpStore) create(ctx context.Context, userID string, input MCPServerInput) (MCPServer, error) {
	input, err := validateMCPInput(input, true)
	if err != nil {
		return MCPServer{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MCPServer{}, fmt.Errorf("start MCP create: %w", err)
	}
	defer tx.Rollback()
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return MCPServer{}, errors.New("user id is required")
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID); err != nil {
		return MCPServer{}, fmt.Errorf("ensure user: %w", err)
	}
	server := MCPServer{ID: uuid.NewString(), Name: input.Name, URL: input.URL, Transport: input.Transport, Enabled: input.Enabled}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_mcp_servers (server_id, user_id, name, url, transport, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, server.ID, userID, server.Name, server.URL, server.Transport, server.Enabled); err != nil {
		return MCPServer{}, fmt.Errorf("create MCP server: %w", err)
	}
	if err := insertHeaders(ctx, tx, server.ID, input.Headers); err != nil {
		return MCPServer{}, err
	}
	if err := tx.Commit(); err != nil {
		return MCPServer{}, fmt.Errorf("commit MCP create: %w", err)
	}
	server.Headers = headersFromInput(input.Headers)
	return server, nil
}

func (s *mcpStore) update(ctx context.Context, userID, serverID string, input MCPServerInput) (MCPServer, error) {
	input, err := validateMCPInput(input, false)
	if err != nil {
		return MCPServer{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MCPServer{}, fmt.Errorf("start MCP update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE user_mcp_servers
		SET name = ?, url = ?, transport = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND server_id = ?
	`, input.Name, input.URL, input.Transport, input.Enabled, strings.TrimSpace(userID), strings.TrimSpace(serverID))
	if err != nil {
		return MCPServer{}, fmt.Errorf("update MCP server: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return MCPServer{}, fmt.Errorf("check MCP update: %w", err)
	}
	if changed == 0 {
		return MCPServer{}, ErrMCPServerNotFound
	}
	existing, err := loadHeadersTx(ctx, tx, serverID)
	if err != nil {
		return MCPServer{}, err
	}
	if err := replaceHeaders(ctx, tx, serverID, existing, input.Headers); err != nil {
		return MCPServer{}, err
	}
	if err := tx.Commit(); err != nil {
		return MCPServer{}, fmt.Errorf("commit MCP update: %w", err)
	}
	return s.loadOne(ctx, userID, serverID)
}

func (s *mcpStore) delete(ctx context.Context, userID, serverID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start MCP delete: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_servers WHERE user_id = ? AND server_id = ?`, strings.TrimSpace(userID), strings.TrimSpace(serverID))
	if err != nil {
		return fmt.Errorf("delete MCP server: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check MCP delete: %w", err)
	}
	if changed == 0 {
		return ErrMCPServerNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_headers WHERE server_id = ?`, strings.TrimSpace(serverID)); err != nil {
		return fmt.Errorf("delete MCP headers: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit MCP delete: %w", err)
	}
	return nil
}

func (s *mcpStore) loadOne(ctx context.Context, userID, serverID string) (MCPServer, error) {
	var server MCPServer
	err := s.db.QueryRowContext(ctx, `
		SELECT server_id, name, url, transport, enabled
		FROM user_mcp_servers WHERE user_id = ? AND server_id = ?
	`, strings.TrimSpace(userID), strings.TrimSpace(serverID)).Scan(&server.ID, &server.Name, &server.URL, &server.Transport, &server.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return MCPServer{}, ErrMCPServerNotFound
	}
	if err != nil {
		return MCPServer{}, fmt.Errorf("load MCP server: %w", err)
	}
	headers, err := s.loadHeaders(ctx, server.ID)
	if err != nil {
		return MCPServer{}, err
	}
	server.Headers = headers
	return server, nil
}
