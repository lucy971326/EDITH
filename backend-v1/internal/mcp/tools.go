// Package mcp turns one user's stored remote MCP configurations into tools for
// one Agent run. It owns only per-run MCP connections, never shared state.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	toolmcp "trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// OpenTools connects every supplied MCP server, lists its tools, and returns
// both the tools and the cleanup function for this one run.
func OpenTools(ctx context.Context, servers []userconfig.MCPServer) ([]tool.Tool, func() error, error) {
	additionalTools := []tool.Tool{}
	toolSets := []*toolmcp.ToolSet{}

	for _, server := range servers {
		if err := validateRemoteTarget(ctx, server.URL); err != nil {
			closeToolSets(toolSets)
			return nil, nil, fmt.Errorf("validate MCP server %q: %w", server.Name, err)
		}

		toolSet := toolmcp.NewMCPToolSet(toolmcp.ConnectionConfig{
			Transport: server.Transport,
			ServerURL: server.URL,
			Headers:   headers(server.Headers),
		}, toolmcp.WithName(toolSetName(server)))
		if err := toolSet.Init(ctx); err != nil {
			closeToolSets(append(toolSets, toolSet))
			return nil, nil, fmt.Errorf("open MCP server %q: %w", server.Name, err)
		}

		toolSets = append(toolSets, toolSet)
		additionalTools = append(additionalTools, toolSet.Tools(ctx)...)
	}

	return additionalTools, func() error {
		return closeToolSets(toolSets)
	}, nil
}

func headers(values []userconfig.MCPHeader) map[string]string {
	headers := make(map[string]string, len(values))
	for _, header := range values {
		headers[header.Name] = header.Value
	}
	return headers
}

func closeToolSets(toolSets []*toolmcp.ToolSet) error {
	var errs []error
	for index := len(toolSets) - 1; index >= 0; index-- {
		if err := toolSets[index].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func toolSetName(server userconfig.MCPServer) string {
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

func validateRemoteTarget(ctx context.Context, rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Hostname() == "" {
		return errors.New("invalid MCP URL")
	}
	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("local MCP URLs are not allowed")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve MCP host: %w", err)
	}
	for _, address := range addresses {
		ip := net.ParseIP(address.String())
		if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
			return errors.New("MCP host resolves to a local address")
		}
	}
	return nil
}
