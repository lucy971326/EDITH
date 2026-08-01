package userconfig

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *mcpStore) loadHeaders(ctx context.Context, serverID string) ([]MCPHeader, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT header_name, header_value FROM user_mcp_headers WHERE server_id = ? ORDER BY header_name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("load MCP headers: %w", err)
	}
	defer rows.Close()
	headers := []MCPHeader{}
	for rows.Next() {
		var header MCPHeader
		if err := rows.Scan(&header.Name, &header.Value); err != nil {
			return nil, fmt.Errorf("scan MCP header: %w", err)
		}
		headers = append(headers, header)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MCP headers: %w", err)
	}
	return headers, nil
}

func loadHeadersTx(ctx context.Context, tx *sql.Tx, serverID string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT header_name, header_value FROM user_mcp_headers WHERE server_id = ?`, serverID)
	if err != nil {
		return nil, fmt.Errorf("load MCP headers: %w", err)
	}
	defer rows.Close()
	existing := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan MCP header: %w", err)
		}
		existing[name] = value
	}
	return existing, rows.Err()
}

func insertHeaders(ctx context.Context, tx *sql.Tx, serverID string, headers []MCPHeaderInput) error {
	for _, header := range headers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_mcp_headers (server_id, header_name, header_value) VALUES (?, ?, ?)`, serverID, header.Name, *header.Value); err != nil {
			return fmt.Errorf("insert MCP header: %w", err)
		}
	}
	return nil
}

func replaceHeaders(ctx context.Context, tx *sql.Tx, serverID string, existing map[string]string, headers []MCPHeaderInput) error {
	submitted := make(map[string]MCPHeaderInput, len(headers))
	for _, header := range headers {
		submitted[header.Name] = header
	}
	for name := range existing {
		if _, found := submitted[name]; !found {
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_mcp_headers WHERE server_id = ? AND header_name = ?`, serverID, name); err != nil {
				return fmt.Errorf("delete MCP header: %w", err)
			}
		}
	}
	for _, header := range headers {
		if header.Value == nil {
			if _, found := existing[header.Name]; !found {
				return fmt.Errorf("%w: new header %q requires a value", ErrInvalidMCPServer, header.Name)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_mcp_headers (server_id, header_name, header_value) VALUES (?, ?, ?) ON CONFLICT(server_id, header_name) DO UPDATE SET header_value = excluded.header_value`, serverID, header.Name, *header.Value); err != nil {
			return fmt.Errorf("save MCP header: %w", err)
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
