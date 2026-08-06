package engine

import (
	"errors"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Dependencies 是 Engine 持有的长期依赖。
type Dependencies struct {
	// WorkspaceID 是当前项目的 Session 隔离身份。
	WorkspaceID string
	// Runner 是 Workspace 已组装好的 Agent 运行能力。
	Runner runner.ManagedRunner
}

// Engine 管理一个本地项目的 Agent Runner。
type Engine struct {
	// workspaceID 是当前项目的稳定身份；它隔离不同项目的会话。
	workspaceID string
	// runner 是已组装的 Agent 运行能力；它负责执行和取消一次 Run。
	runner runner.ManagedRunner
	// runningMu 保护下面两份随运行变化的状态，避免并发 Run 相互干扰。
	runningMu sync.Mutex
	// runningSession 记录每个会话当前对应的请求身份，用于限制一个会话只运行一个 Run。
	runningSession map[string]string
	// userCanceled 记录用户主动取消的请求身份，用于在流结束时区分“取消”和“普通结束”。
	userCanceled map[string]struct{}
}

// New 使用已经创建好的长期依赖组装 Engine。
func New(dependencies Dependencies) (*Engine, error) {
	if strings.TrimSpace(dependencies.WorkspaceID) == "" || dependencies.Runner == nil {
		return nil, errors.New("engine dependencies are incomplete")
	}
	return &Engine{
		workspaceID:    dependencies.WorkspaceID,
		runner:         dependencies.Runner,
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

// Close 关闭 Engine 持有的 Runner。
func (e *Engine) Close() error {
	return e.runner.Close()
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
