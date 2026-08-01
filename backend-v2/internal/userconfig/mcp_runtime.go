package userconfig

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	toolmcp "trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// OpenTools 连接用户已启用的 MCP 服务，返回本次运行的工具和关闭函数。
func (m *MCP) OpenTools(ctx context.Context, userID string) ([]tool.Tool, func() error, error) {
	servers, err := m.LoadEnabled(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	tools := []tool.Tool{}
	toolSets := []*toolmcp.ToolSet{}
	for _, server := range servers {
		if err := validateRemoteMCPHost(ctx, server.URL); err != nil {
			_ = closeMCPToolSets(toolSets)
			return nil, nil, fmt.Errorf("validate MCP server %q: %w", server.Name, err)
		}
		toolSet := toolmcp.NewMCPToolSet(toolmcp.ConnectionConfig{
			Transport: server.Transport,
			ServerURL: server.URL,
			Headers:   mcpHeaders(server.Headers),
		}, toolmcp.WithName(mcpToolSetName(server)))
		if err := toolSet.Init(ctx); err != nil {
			_ = closeMCPToolSets(append(toolSets, toolSet))
			return nil, nil, fmt.Errorf("open MCP server %q: %w", server.Name, err)
		}
		toolSets = append(toolSets, toolSet)
		tools = append(tools, toolSet.Tools(ctx)...)
	}
	return tools, func() error { return closeMCPToolSets(toolSets) }, nil
}

func validateRemoteMCPHost(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("invalid MCP URL")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("local MCP URLs are not allowed")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("lookup MCP host: %w", err)
	}
	for _, address := range addresses {
		ip := net.ParseIP(address.String())
		if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
			return errors.New("MCP host resolves to a local address")
		}
	}
	return nil
}

func mcpHeaders(values []MCPHeader) map[string]string {
	headers := make(map[string]string, len(values))
	for _, header := range values {
		headers[header.Name] = header.Value
	}
	return headers
}

func mcpToolSetName(server MCPServer) string {
	var name strings.Builder
	for _, character := range strings.ToLower(server.Name) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			name.WriteRune(character)
			continue
		}
		if (character == '-' || character == '_' || unicode.IsSpace(character)) && name.Len() > 0 {
			name.WriteByte('_')
		}
	}
	prefix := strings.Trim(name.String(), "_")
	if prefix == "" {
		prefix = "mcp"
	}
	shortID := strings.ReplaceAll(server.ID, "-", "")
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return prefix + "_" + shortID
}

func closeMCPToolSets(toolSets []*toolmcp.ToolSet) error {
	var closeErrors []error
	for index := len(toolSets) - 1; index >= 0; index-- {
		if err := toolSets[index].Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}
