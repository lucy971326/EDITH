// Package workspace 组装并持有一个 ProjectRoot 的长期产品能力。
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"edith/studio/internal/engine"
	"edith/studio/internal/models"
	"edith/studio/internal/project"
	"edith/studio/internal/session"
	"edith/studio/internal/tools"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
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
	// Project 是读取当前项目文件树和文件内容的能力。
	Project *project.Module
	// Sessions 是框架 SessionService 的长期所有者。
	Sessions *session.Module
	// Models 是启动期创建的模型实例目录和公开能力描述。
	Models *models.Module

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
	sessionModule, err := session.Create()
	if err != nil {
		closeToolSets(toolSets)
		return nil, fmt.Errorf("create session module: %w", err)
	}
	agentRuntime := llmagent.New(
		agentName,
		llmagent.WithModel(modelModule.DefaultModel()),
		llmagent.WithModels(modelModule.AgentModels()),
		llmagent.WithGlobalInstruction(systemPrompt),
		llmagent.WithToolSets(toolSets),
	)
	managedRunner, ok := runner.NewRunner(
		appName,
		agentRuntime,
		runner.WithSessionService(sessionModule.Service()),
	).(runner.ManagedRunner)
	if !ok {
		closeToolSets(toolSets)
		_ = sessionModule.Close()
		return nil, errors.New("framework runner does not support cancellation")
	}
	currentWorkspaceID := workspaceID(projectRoot)
	engineRuntime, err := engine.New(
		engine.Dependencies{
			WorkspaceID: currentWorkspaceID,
			Runner:      managedRunner,
			Models:      modelModule,
		})
	if err != nil {
		_ = managedRunner.Close()
		closeToolSets(toolSets)
		_ = sessionModule.Close()
		return nil, fmt.Errorf("create engine: %w", err)
	}
	return &Workspace{
		ProjectRoot: projectRoot,
		WorkspaceID: currentWorkspaceID,
		Engine:      engineRuntime,
		Project:     projectModule,
		Sessions:    sessionModule,
		Models:      modelModule,
		toolSets:    toolSets,
	}, nil
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

func closeToolSets(toolSets []tool.ToolSet) {
	for _, toolSet := range toolSets {
		_ = toolSet.Close()
	}
}
