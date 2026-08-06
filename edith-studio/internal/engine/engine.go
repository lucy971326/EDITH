package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"edith/studio/internal/models"
	"edith/studio/internal/session"
	"edith/studio/internal/tools"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessionpkg "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const (
	appName      = "edith-studio"
	agentName    = "edith"
	systemPrompt = "You are EDITH, a careful coding agent. Work only in the current project. Explain important changes clearly and keep code readable."
)

// Dependencies 是 Engine 持有的长期依赖。
type Dependencies struct {
	ProjectRoot    string
	Runner         runner.ManagedRunner
	SessionService sessionpkg.Service
	ToolSets       []tool.ToolSet
}

// Engine 管理一个本地项目的 Agent Runner。
type Engine struct {
	// workspaceID 是当前项目的稳定身份；它隔离不同项目的会话。
	workspaceID string
	// runner 是已组装的 Agent 运行能力；它负责执行和取消一次 Run。
	runner runner.ManagedRunner
	// sessionService 是已组装的会话持久化能力；Runner 用它保存对话。
	sessionService sessionpkg.Service
	// toolSets 是已组装的工具能力；Agent 在 Run 中可调用其中的工具。
	toolSets []tool.ToolSet

	// runningMu 保护下面两份随运行变化的状态，避免并发 Run 相互干扰。
	runningMu sync.Mutex
	// runningSession 记录每个会话当前对应的请求身份，用于限制一个会话只运行一个 Run。
	runningSession map[string]string
	// userCanceled 记录用户主动取消的请求身份，用于在流结束时区分“取消”和“普通结束”。
	userCanceled map[string]struct{}
}

// Open 为当前项目组装完整的 Agent 内核。
func Open(projectRoot string) (*Engine, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	projectRoot = filepath.Clean(projectRoot)
	modelRegistry, err := models.Open()
	if err != nil {
		return nil, err
	}
	toolSets, err := tools.NewToolSets(projectRoot)
	if err != nil {
		return nil, err
	}
	sessionService, err := session.Open()
	if err != nil {
		closeToolSets(toolSets)
		return nil, err
	}
	agent := llmagent.New(
		agentName,
		llmagent.WithModel(modelRegistry.Default),
		llmagent.WithModels(modelRegistry.All),
		llmagent.WithGlobalInstruction(systemPrompt),
		llmagent.WithToolSets(toolSets),
	)
	managedRunner, ok := runner.NewRunner(
		appName,
		agent,
		runner.WithSessionService(sessionService),
	).(runner.ManagedRunner)
	if !ok {
		closeToolSets(toolSets)
		_ = sessionService.Close()
		return nil, errors.New("framework runner does not support cancellation")
	}
	return New(Dependencies{
		ProjectRoot:    projectRoot,
		Runner:         managedRunner,
		SessionService: sessionService,
		ToolSets:       toolSets,
	})
}

// New 使用已经创建好的长期依赖组装 Engine。
func New(dependencies Dependencies) (*Engine, error) {
	if dependencies.ProjectRoot == "" || dependencies.Runner == nil || dependencies.SessionService == nil {
		return nil, errors.New("engine dependencies are incomplete")
	}
	return &Engine{
		workspaceID:    workspaceID(dependencies.ProjectRoot),
		runner:         dependencies.Runner,
		sessionService: dependencies.SessionService,
		toolSets:       dependencies.ToolSets,
		runningSession: make(map[string]string),
		userCanceled:   make(map[string]struct{}),
	}, nil
}

// ValidateInput 检查一次 Run 的必要输入。
func ValidateInput(input RunInput) error {
	if strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Message) == "" {
		return ErrInvalidInput
	}
	return nil
}

// Cancel 按 requestID 请求框架停止任务。
func (e *Engine) Cancel(requestID string) bool {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false
	}
	e.runningMu.Lock()
	e.userCanceled[requestID] = struct{}{}
	e.runningMu.Unlock()
	if e.runner.Cancel(requestID) {
		return true
	}
	e.clearUserCanceled(requestID)
	return false
}

// Close 释放 Engine 持有的 Runner、ToolSet 和 SessionService。
func (e *Engine) Close() error {
	var closeErrors []error
	if err := e.runner.Close(); err != nil {
		closeErrors = append(closeErrors, err)
	}
	for _, toolSet := range e.toolSets {
		if err := toolSet.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if err := e.sessionService.Close(); err != nil {
		closeErrors = append(closeErrors, err)
	}
	return errors.Join(closeErrors...)
}

func (e *Engine) reserveSession(input RunInput) error {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	if _, exists := e.runningSession[input.SessionID]; exists {
		return ErrSessionBusy
	}
	e.runningSession[input.SessionID] = input.RequestID
	return nil
}

func (e *Engine) releaseSession(sessionID string) {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	delete(e.runningSession, sessionID)
}

func (e *Engine) wasUserCanceled(requestID string) bool {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	_, exists := e.userCanceled[requestID]
	return exists
}

func (e *Engine) clearUserCanceled(requestID string) {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	delete(e.userCanceled, requestID)
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
