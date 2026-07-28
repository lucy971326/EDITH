package userconfig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrInvalidMCPServer means the submitted MCP configuration cannot be stored.
	ErrInvalidMCPServer = errors.New("invalid MCP server")
	// ErrMCPServerNotFound means the server either does not exist or belongs to another user.
	ErrMCPServerNotFound = errors.New("MCP server not found")
)

var headerNamePattern = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

// MCPHeader contains the actual value and must stay server-side.
type MCPHeader struct {
	Name  string
	Value string
}

// MCPServer is one user's complete MCP configuration, including secret headers.
type MCPServer struct {
	ID        string
	Name      string
	URL       string
	Transport string
	Enabled   bool
	Headers   []MCPHeader
}

// MCPHeaderInput preserves an existing value when Value is nil.
type MCPHeaderInput struct {
	Name  string
	Value *string
}

// MCPServerInput is the editable part of an MCP server. The Store generates ID.
type MCPServerInput struct {
	Name      string
	URL       string
	Transport string
	Enabled   bool
	Headers   []MCPHeaderInput
}

// ListMCPServers returns one user's complete MCP configuration for server-side use.
func (s *Store) ListMCPServers(ctx context.Context, userID string) ([]MCPServer, error) {
	return s.loadMCPServers(ctx, userID, false)
}

// LoadEnabledMCPServers returns only the configurations that may be connected
// during one Agent run. Header values stay inside the Go runtime.
func (s *Store) LoadEnabledMCPServers(ctx context.Context, userID string) ([]MCPServer, error) {
	return s.loadMCPServers(ctx, userID, true)
}

func (s *Store) loadMCPServers(ctx context.Context, userID string, enabledOnly bool) ([]MCPServer, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return nil, err
	}
	query := `
		SELECT server_id, name, url, transport, enabled
		FROM user_mcp_servers
		WHERE user_id = ?`
	if enabledOnly {
		query += " AND enabled = 1"
	}
	query += `
		ORDER BY created_at, server_id
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers %q: %w", userID, err)
	}
	defer rows.Close()

	servers := []MCPServer{}
	for rows.Next() {
		var server MCPServer
		if err := rows.Scan(&server.ID, &server.Name, &server.URL, &server.Transport, &server.Enabled); err != nil {
			return nil, fmt.Errorf("scan MCP server %q: %w", userID, err)
		}
		headers, err := s.loadMCPHeaders(ctx, server.ID)
		if err != nil {
			return nil, err
		}
		server.Headers = headers
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MCP servers %q: %w", userID, err)
	}
	return servers, nil
}

// CreateMCPServer stores a new MCP server and generates its server-owned ID.
func (s *Store) CreateMCPServer(ctx context.Context, userID string, input MCPServerInput) (MCPServer, error) {
	input, err := validateMCPServerInput(input, true)
	if err != nil {
		return MCPServer{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MCPServer{}, fmt.Errorf("start create MCP server %q: %w", userID, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO users (user_id) VALUES (?)`, userID); err != nil {
		return MCPServer{}, fmt.Errorf("ensure user %q: %w", userID, err)
	}

	server := MCPServer{ID: uuid.NewString(), Name: input.Name, URL: input.URL, Transport: input.Transport, Enabled: input.Enabled}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_mcp_servers (server_id, user_id, name, url, transport, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, server.ID, userID, server.Name, server.URL, server.Transport, server.Enabled); err != nil {
		return MCPServer{}, fmt.Errorf("create MCP server %q/%q: %w", userID, server.ID, err)
	}
	if err := insertMCPHeaders(ctx, tx, server.ID, input.Headers); err != nil {
		return MCPServer{}, err
	}
	if err := tx.Commit(); err != nil {
		return MCPServer{}, fmt.Errorf("commit create MCP server %q/%q: %w", userID, server.ID, err)
	}
	server.Headers = headersFromInput(input.Headers)
	return server, nil
}

// UpdateMCPServer updates one server owned by userID. Omitted existing headers
// are deleted; a submitted header with nil Value keeps its stored value.
func (s *Store) UpdateMCPServer(ctx context.Context, userID, serverID string, input MCPServerInput) (MCPServer, error) {
	input, err := validateMCPServerInput(input, false)
	if err != nil {
		return MCPServer{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MCPServer{}, fmt.Errorf("start update MCP server %q/%q: %w", userID, serverID, err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE user_mcp_servers
		SET name = ?, url = ?, transport = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND server_id = ?
	`, input.Name, input.URL, input.Transport, input.Enabled, userID, serverID)
	if err != nil {
		return MCPServer{}, fmt.Errorf("update MCP server %q/%q: %w", userID, serverID, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return MCPServer{}, fmt.Errorf("check MCP server %q/%q: %w", userID, serverID, err)
	}
	if changed == 0 {
		return MCPServer{}, ErrMCPServerNotFound
	}

	existing, err := loadMCPHeadersTx(ctx, tx, serverID)
	if err != nil {
		return MCPServer{}, err
	}
	if err := replaceMCPHeaders(ctx, tx, serverID, existing, input.Headers); err != nil {
		return MCPServer{}, err
	}
	if err := tx.Commit(); err != nil {
		return MCPServer{}, fmt.Errorf("commit update MCP server %q/%q: %w", userID, serverID, err)
	}

	return s.loadMCPServer(ctx, userID, serverID)
}

// DeleteMCPServer removes one server and its headers. It never crosses user boundaries.
func (s *Store) DeleteMCPServer(ctx context.Context, userID, serverID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start delete MCP server %q/%q: %w", userID, serverID, err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_servers WHERE user_id = ? AND server_id = ?`, userID, serverID)
	if err != nil {
		return fmt.Errorf("delete MCP server %q/%q: %w", userID, serverID, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted MCP server %q/%q: %w", userID, serverID, err)
	}
	if changed == 0 {
		return ErrMCPServerNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_headers WHERE server_id = ?`, serverID); err != nil {
		return fmt.Errorf("delete MCP headers %q: %w", serverID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete MCP server %q/%q: %w", userID, serverID, err)
	}
	return nil
}

func (s *Store) loadMCPServer(ctx context.Context, userID, serverID string) (MCPServer, error) {
	var server MCPServer
	err := s.db.QueryRowContext(ctx, `
		SELECT server_id, name, url, transport, enabled
		FROM user_mcp_servers WHERE user_id = ? AND server_id = ?
	`, userID, serverID).Scan(&server.ID, &server.Name, &server.URL, &server.Transport, &server.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return MCPServer{}, ErrMCPServerNotFound
	}
	if err != nil {
		return MCPServer{}, fmt.Errorf("load MCP server %q/%q: %w", userID, serverID, err)
	}
	headers, err := s.loadMCPHeaders(ctx, serverID)
	if err != nil {
		return MCPServer{}, err
	}
	server.Headers = headers
	return server, nil
}

func (s *Store) loadMCPHeaders(ctx context.Context, serverID string) ([]MCPHeader, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT header_name, header_value FROM user_mcp_headers WHERE server_id = ? ORDER BY header_name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("load MCP headers %q: %w", serverID, err)
	}
	defer rows.Close()
	headers := []MCPHeader{}
	for rows.Next() {
		var header MCPHeader
		if err := rows.Scan(&header.Name, &header.Value); err != nil {
			return nil, fmt.Errorf("scan MCP header %q: %w", serverID, err)
		}
		headers = append(headers, header)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MCP headers %q: %w", serverID, err)
	}
	return headers, nil
}

func loadMCPHeadersTx(ctx context.Context, tx *sql.Tx, serverID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT header_name, header_value FROM user_mcp_headers WHERE server_id = ?`, serverID)
	if err != nil {
		return nil, fmt.Errorf("load MCP headers %q: %w", serverID, err)
	}
	defer rows.Close()
	headers := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan MCP header %q: %w", serverID, err)
		}
		headers[name] = value
	}
	return headers, rows.Err()
}

func insertMCPHeaders(ctx context.Context, tx *sql.Tx, serverID string, headers []MCPHeaderInput) error {
	for _, header := range headers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_mcp_headers (server_id, header_name, header_value) VALUES (?, ?, ?)`, serverID, header.Name, *header.Value); err != nil {
			return fmt.Errorf("insert MCP header %q: %w", header.Name, err)
		}
	}
	return nil
}

func replaceMCPHeaders(ctx context.Context, tx *sql.Tx, serverID string, existing map[string]string, headers []MCPHeaderInput) error {
	submitted := map[string]MCPHeaderInput{}
	for _, header := range headers {
		submitted[header.Name] = header
	}
	for name := range existing {
		if _, ok := submitted[name]; !ok {
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_headers WHERE server_id = ? AND header_name = ?`, serverID, name); err != nil {
				return fmt.Errorf("delete MCP header %q: %w", name, err)
			}
		}
	}
	for _, header := range headers {
		if header.Value == nil {
			if _, ok := existing[header.Name]; !ok {
				return fmt.Errorf("%w: new header %q requires a value", ErrInvalidMCPServer, header.Name)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_mcp_headers (server_id, header_name, header_value)
			VALUES (?, ?, ?)
			ON CONFLICT(server_id, header_name) DO UPDATE SET header_value = excluded.header_value
		`, serverID, header.Name, *header.Value); err != nil {
			return fmt.Errorf("save MCP header %q: %w", header.Name, err)
		}
	}
	return nil
}

func headersFromInput(headers []MCPHeaderInput) []MCPHeader {
	values := make([]MCPHeader, 0, len(headers))
	for _, header := range headers {
		values = append(values, MCPHeader{Name: header.Name, Value: *header.Value})
	}
	return values
}

func validateMCPServerInput(input MCPServerInput, requireHeaderValues bool) (MCPServerInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	if input.Name == "" {
		return MCPServerInput{}, fmt.Errorf("%w: name is required", ErrInvalidMCPServer)
	}
	if input.Transport != "streamable" && input.Transport != "sse" {
		return MCPServerInput{}, fmt.Errorf("%w: unsupported transport", ErrInvalidMCPServer)
	}
	parsedURL, err := url.ParseRequestURI(input.URL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return MCPServerInput{}, fmt.Errorf("%w: URL must be HTTP or HTTPS", ErrInvalidMCPServer)
	}
	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return MCPServerInput{}, fmt.Errorf("%w: local MCP URLs are not allowed", ErrInvalidMCPServer)
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return MCPServerInput{}, fmt.Errorf("%w: local MCP URLs are not allowed", ErrInvalidMCPServer)
	}

	seen := map[string]bool{}
	for index := range input.Headers {
		header := &input.Headers[index]
		header.Name = strings.TrimSpace(header.Name)
		if !headerNamePattern.MatchString(header.Name) {
			return MCPServerInput{}, fmt.Errorf("%w: invalid header name", ErrInvalidMCPServer)
		}
		header.Name = textproto.CanonicalMIMEHeaderKey(header.Name)
		key := strings.ToLower(header.Name)
		if seen[key] {
			return MCPServerInput{}, fmt.Errorf("%w: duplicate header name", ErrInvalidMCPServer)
		}
		seen[key] = true
		if header.Value != nil {
			value := strings.TrimSpace(*header.Value)
			if value == "" {
				header.Value = nil
			} else {
				header.Value = &value
			}
		}
		if requireHeaderValues && header.Value == nil {
			return MCPServerInput{}, fmt.Errorf("%w: new header requires a value", ErrInvalidMCPServer)
		}
	}
	return input, nil
}
