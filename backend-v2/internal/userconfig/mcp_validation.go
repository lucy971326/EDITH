package userconfig

import (
	"fmt"
	"net"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
)

var headerNamePattern = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

func validateMCPInput(input MCPServerInput, requireHeaderValues bool) (MCPServerInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	input.Transport = strings.TrimSpace(input.Transport)
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
