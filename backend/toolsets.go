package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"demo/sandbox"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
)

// loadGitHubToolSet 连接 GitHub MCP，并加载它提供的工具。
func loadGitHubToolSet(ctx context.Context) (*mcp.ToolSet, error) {
	githubToken, err := requiredEnv("GITHUB_TOKEN")
	if err != nil {
		return nil, err
	}

	githubToolSet := mcp.NewMCPToolSet(
		mcp.ConnectionConfig{
			Transport: "streamable_http",
			ServerURL: "https://api.githubcopilot.com/mcp/",
			Timeout:   30 * time.Second,
			Headers: map[string]string{
				"Authorization":  "Bearer " + githubToken,
				"X-MCP-Toolsets": "default",
			},
		},
		mcp.WithName("github"),
	)
	if err := githubToolSet.Init(ctx); err != nil {
		githubToolSet.Close()
		return nil, fmt.Errorf("initialize GitHub MCP: %w", err)
	}

	return githubToolSet, nil
}

// newSandboxToolSet 把同一个工作区 Provider 交给 Agent 工具。
// 上传接口也使用它，确保文件落到 Agent 实际工作的那个工作区。
func newSandboxToolSet(provider sandbox.BackendProvider) tool.ToolSet {
	return sandbox.NewToolSet(provider)
}

// newBackendProvider 根据配置创建每个会话工作区使用的执行后端。
func newBackendProvider(db *sql.DB) sandbox.BackendProvider {
	if os.Getenv("SANDBOX_MODE") == "e2b" {
		provider, err := sandbox.NewE2BProvider(db, sandbox.E2BProviderOptions{
			APIKey:   envOr("E2B_API_KEY", ""),
			Domain:   envOr("E2B_DOMAIN", ""),
			Template: envOr("E2B_TEMPLATE", "base"),
			Timeout:  10 * time.Minute,
		})
		if err != nil {
			log.Fatalf("new E2B backend provider: %v", err)
		}
		return provider
	}

	provider, err := sandbox.NewLocalProvider("./workspace", "./skills/system")
	if err != nil {
		log.Fatalf("new local backend provider: %v", err)
	}
	return provider
}
