package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	toolmcp "trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// Dependencies 是创建 MCP 模块所需的稳定项目配置。
type Dependencies struct {
	// ProjectRoot 是本次 Studio 进程服务的项目根目录，用于读取项目级 mcp.json。
	ProjectRoot string
}

// Module 持有全部 MCP server 的 ToolSet 和运行状态。
// 单个 server 连接失败只记录 error 状态，不阻塞其他 server 或 Studio 启动。
type Module struct {
	// toolSets 是连接成功的 MCP ToolSet，供 Workspace 汇入 Agent。
	toolSets []tool.ToolSet
	// statuses 是每个已配置 server 的运行状态，供 Studio 和 Web 展示。
	statuses []ServerStatus
}

// New 读取配置、注入 stdio 环境变量、建立连接并预热工具列表。
func New(dependencies Dependencies) (*Module, error) {
	config, err := Load(dependencies.ProjectRoot)
	if err != nil {
		return nil, err
	}

	module := &Module{}
	names := make([]string, 0, len(config.Servers))
	for name := range config.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		module.addServer(name, config.Servers[name])
	}
	return module, nil
}

// addServer 为单个 server 建立连接并记录状态；失败只进状态，不返回错误。
func (m *Module) addServer(name string, serverConfig ServerConfig) {
	status := ServerStatus{Name: name, Transport: serverConfig.Transport}
	conn, err := connectionConfig(name, serverConfig)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		m.statuses = append(m.statuses, status)
		return
	}

	// stdio 子进程继承父进程环境变量；框架不支持在配置里透传 env，
	// 所以这里把 stdio 的 env 注入当前进程，子进程启动时自动带上。
	// 副作用：注入的 env 随 EDITH 进程存活，同进程其他子进程也能读到。
	if strings.EqualFold(conn.Transport, "stdio") {
		injectEnv(serverConfig.Env)
	}

	toolSet := toolmcp.NewMCPToolSet(conn, toolmcp.WithName(name))
	if err := toolSet.Init(context.Background()); err != nil {
		status.Status = "error"
		status.Error = fmt.Sprintf("connect: %v", err)
		m.statuses = append(m.statuses, status)
		return
	}
	status.Status = "connected"
	status.ToolCount = len(toolSet.Tools(context.Background()))
	m.toolSets = append(m.toolSets, toolSet)
	m.statuses = append(m.statuses, status)
}

// connectionConfig 校验配置并组装框架连接参数；timeout 按字符串解析。
func connectionConfig(name string, serverConfig ServerConfig) (toolmcp.ConnectionConfig, error) {
	transport := strings.ToLower(strings.TrimSpace(serverConfig.Transport))
	conn := toolmcp.ConnectionConfig{
		Transport: transport,
		ServerURL: serverConfig.ServerURL,
		Headers:   serverConfig.Headers,
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
	}
	if serverConfig.Timeout != "" {
		duration, err := time.ParseDuration(serverConfig.Timeout)
		if err != nil {
			return toolmcp.ConnectionConfig{}, fmt.Errorf("server %q: invalid timeout %q", name, serverConfig.Timeout)
		}
		conn.Timeout = duration
	}
	switch transport {
	case "stdio":
		if strings.TrimSpace(conn.Command) == "" {
			return toolmcp.ConnectionConfig{}, fmt.Errorf("server %q: stdio transport requires command", name)
		}
	case "sse", "streamable", "streamable_http":
		if strings.TrimSpace(conn.ServerURL) == "" {
			return toolmcp.ConnectionConfig{}, fmt.Errorf("server %q: %s transport requires serverUrl", name, transport)
		}
	default:
		return toolmcp.ConnectionConfig{}, fmt.Errorf("server %q: unsupported transport %q", name, serverConfig.Transport)
	}
	return conn, nil
}

func injectEnv(env map[string]string) {
	for key, value := range env {
		_ = os.Setenv(key, value)
	}
}

// ToolSets 返回连接成功的 MCP ToolSet，供 Workspace 汇入 llmagent.WithToolSets。
func (m *Module) ToolSets() []tool.ToolSet {
	return m.toolSets
}

// Status 返回所有已配置 server 的运行状态，供 Studio 和 Web 展示。
func (m *Module) Status() []ServerStatus {
	return m.statuses
}

// Close 关闭全部 MCP ToolSet 持有的连接。
func (m *Module) Close() error {
	var closeErrors []error
	for _, toolSet := range m.toolSets {
		closeErrors = append(closeErrors, toolSet.Close())
	}
	return errors.Join(closeErrors...)
}
