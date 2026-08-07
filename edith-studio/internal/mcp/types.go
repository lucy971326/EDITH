// Package mcp 管理用户级与项目级 MCP server 的连接、工具集和状态。
package mcp

import "errors"

// ErrInvalidConfig 表示 mcp.json 配置无法解析或字段不合法。
var ErrInvalidConfig = errors.New("invalid mcp config")

// Config 是 mcp.json 的完整格式。
type Config struct {
	// Servers 保存 server 名称到连接配置的映射。
	Servers map[string]ServerConfig `json:"servers"`
}

// ServerConfig 是一个 MCP server 的连接配置。
// 密钥允许直接写入 Env/Headers；EDITH 启动时把它们注入 stdio 子进程或 HTTP 请求。
type ServerConfig struct {
	// Transport 是传输方式：stdio / sse / streamable_http。
	Transport string `json:"transport"`
	// Command 是 stdio 传输的可执行命令。
	Command string `json:"command,omitempty"`
	// Args 是 stdio 命令参数。
	Args []string `json:"args,omitempty"`
	// Env 是 stdio 子进程额外注入的环境变量（如 API key），只对 stdio 生效。
	Env map[string]string `json:"env,omitempty"`
	// ServerURL 是 sse / streamable_http 传输的服务地址。
	ServerURL string `json:"serverUrl,omitempty"`
	// Headers 是 HTTP 传输的静态请求头。
	Headers map[string]string `json:"headers,omitempty"`
	// Timeout 是连接超时，例如 "10s"。
	Timeout string `json:"timeout,omitempty"`
}

// ServerStatus 是暴露给 Studio 和 Web 的单个 server 运行状态。
type ServerStatus struct {
	// Name 是 server 在配置中的名称，也是模型侧工具名的前缀。
	Name string `json:"name"`
	// Transport 是配置的传输方式。
	Transport string `json:"transport"`
	// Status 是运行状态：connected 或 error。
	Status string `json:"status"`
	// ToolCount 是连接成功后列出的工具数量。
	ToolCount int `json:"toolCount"`
	// Error 是连接失败时的错误信息，只在 error 状态出现。
	Error string `json:"error,omitempty"`
}
