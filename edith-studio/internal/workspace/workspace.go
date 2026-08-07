// Package workspace 组装并持有一个 ProjectRoot 的长期产品能力。
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"edith/studio/internal/commands"
	"edith/studio/internal/engine"
	"edith/studio/internal/mcp"
	"edith/studio/internal/models"
	"edith/studio/internal/project"
	"edith/studio/internal/promptlog"
	"edith/studio/internal/session"
	"edith/studio/internal/tools"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const (
	appName      = "edith-studio"
	agentName    = "edith"
	systemPrompt = "You are EDITH, a careful coding agent. Work only in the current project. Explain important changes clearly and keep code readable."
)

// Dependencies 是创建 Workspace 所需的稳定启动配置。
type Dependencies struct {
	// ProjectRoot 是本次 Studio 进程服务的项目根目录。
	ProjectRoot string
}

// Workspace 是一个 ProjectRoot 的长期产品运行对象。
type Workspace struct {
	// ProjectRoot 是当前项目的规范化绝对目录。
	ProjectRoot string
	// WorkspaceID 是当前项目的 Session 隔离身份。
	WorkspaceID string
	// Engine 是执行与取消 Agent Run 的内核能力。
	Engine *engine.Engine
	// Commands 是产品层 Slash Command 的目录和执行能力。
	Commands *commands.Module
	// Project 是读取当前项目文件树和文件内容的能力。
	Project *project.Module
	// Sessions 是框架 SessionService 的长期所有者。
	Sessions *session.Module
	// Models 是启动期创建的模型实例目录和公开能力描述。
	Models *models.Module
	// MCP 是 MCP server 连接与状态的产品能力。
	MCP *mcp.Module

	// toolSets 是 Workspace 创建并关闭的默认 Coding 工具资源，不作为产品接口暴露。
	toolSets []tool.ToolSet
}

// Create 创建 ProjectRoot 的全部长期能力，并返回已组装的 Workspace。
func Create(dependencies Dependencies) (*Workspace, error) {
	projectModule, err := project.New(project.Dependencies{ProjectRoot: dependencies.ProjectRoot})
	if err != nil {
		return nil, fmt.Errorf("create project module: %w", err)
	}
	projectRoot := projectModule.ProjectRoot()
	modelModule, err := models.Load()
	if err != nil {
		return nil, fmt.Errorf("load models: %w", err)
	}
	toolSets, err := tools.NewToolSets(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("create tool sets: %w", err)
	}

	// 从创建第一个长期资源起注册清理：失败时按依赖反向关闭已经创建的部分。
	// 之后新增模块只需在 Close 中补一行，不需要逐条修改失败分支。
	workspaceRuntime := &Workspace{
		ProjectRoot: projectRoot,
		WorkspaceID: workspaceID(projectRoot),
		Project:     projectModule,
		Models:      modelModule,
		toolSets:    toolSets,
	}
	success := false
	defer func() {
		if !success {
			_ = workspaceRuntime.Close()
		}
	}()

	mcpModule, err := mcp.New(mcp.Dependencies{ProjectRoot: projectRoot})
	if err != nil {
		return nil, fmt.Errorf("create mcp module: %w", err)
	}
	workspaceRuntime.MCP = mcpModule

	sessionModule, err := session.Create(func(ctx context.Context) (model.Model, error) {
		selection, ok := models.SelectionFromContext(ctx)
		if !ok {
			return nil, errors.New("summary model selection is missing")
		}
		return modelModule.SummaryModel(selection)
	})
	if err != nil {
		return nil, fmt.Errorf("create session module: %w", err)
	}
	workspaceRuntime.Sessions = sessionModule

	// MCP ToolSet 由 mcp.Module 创建和关闭，只在这里汇入 Agent，不放进 workspace.toolSets。
	agentToolSets := make([]tool.ToolSet, 0, len(toolSets)+len(mcpModule.ToolSets()))
	agentToolSets = append(agentToolSets, toolSets...)
	agentToolSets = append(agentToolSets, mcpModule.ToolSets()...)
	agentRuntime := llmagent.New(
		agentName,
		llmagent.WithModel(modelModule.DefaultModel()),
		llmagent.WithModels(modelModule.AgentModels()),
		llmagent.WithGlobalInstruction(systemPrompt),
		llmagent.WithToolSets(agentToolSets),
		llmagent.WithAddSessionSummary(true),
		// 父 Agent 只读取自己的 FilterKey 及其子视图，不读取整个 Session 的其他视图。
		llmagent.WithMessageBranchFilterMode(llmagent.BranchFilterModePrefix),
		llmagent.WithSessionSummaryInjectionMode(llmagent.SessionSummaryInjectionUser),
		llmagent.WithSyncSummaryIntraRun(true),
		llmagent.WithEnableContextCompaction(true),
	)
	promptLogPlugin, err := promptlog.New()
	if err != nil {
		return nil, fmt.Errorf("create prompt log plugin: %w", err)
	}
	managedRunner, ok := runner.NewRunner(
		appName,
		agentRuntime,
		runner.WithSessionService(sessionModule.Service()),
		runner.WithPlugins(promptLogPlugin),
	).(runner.ManagedRunner)
	if !ok {
		return nil, errors.New("framework runner does not support cancellation")
	}
	workspaceRuntime.Engine, err = engine.New(
		engine.Dependencies{
			WorkspaceID: workspaceRuntime.WorkspaceID,
			FilterKey:   appName,
			Runner:      managedRunner,
			Models:      modelModule,
			Sessions:    sessionModule.Service(),
		})
	if err != nil {
		// Engine 未创建成功时 runner 还没有被 Workspace.Close 接管，需要单独关闭。
		_ = managedRunner.Close()
		return nil, fmt.Errorf("create engine: %w", err)
	}
	commandModule, err := commands.New(commands.Dependencies{Engine: workspaceRuntime.Engine})
	if err != nil {
		return nil, fmt.Errorf("create commands module: %w", err)
	}
	workspaceRuntime.Commands = commandModule
	success = true
	return workspaceRuntime, nil
}

// Close 按反向依赖顺序关闭 Workspace 创建的长期资源。
func (w *Workspace) Close() error {
	var closeErrors []error
	if w.Engine != nil {
		closeErrors = append(closeErrors, w.Engine.Close())
	}
	for _, toolSet := range w.toolSets {
		closeErrors = append(closeErrors, toolSet.Close())
	}
	if w.Sessions != nil {
		closeErrors = append(closeErrors, w.Sessions.Close())
	}
	if w.MCP != nil {
		closeErrors = append(closeErrors, w.MCP.Close())
	}
	return errors.Join(closeErrors...)
}

func workspaceID(projectRoot string) string {
	canonicalPath := filepath.Clean(projectRoot)
	if runtime.GOOS == "windows" {
		canonicalPath = strings.ToLower(canonicalPath)
	}
	hash := sha256.Sum256([]byte(canonicalPath))
	return "workspace:" + hex.EncodeToString(hash[:])
}
