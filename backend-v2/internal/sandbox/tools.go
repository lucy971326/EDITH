package sandbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/eric642/e2b-go-sdk"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const workspaceStateKey = "edith:sandbox-workspace"

// toolSet 是 Sandbox 自己提供的文件与进程工具集合。
type toolSet struct{ workspaces *service }

func (s *toolSet) Name() string { return "sandbox" }
func (s *toolSet) Close() error { return nil }
func (s *toolSet) Tools(context.Context) []tool.Tool {
	return []tool.Tool{s.listFilesTool(), s.readFileTool(), s.writeFileTool(), s.makeDirectoryTool(), s.movePathTool(), s.deletePathTool(), s.runCommandTool(), s.startProcessTool(), s.listProcessesTool(), s.killProcessTool()}
}

// currentWorkspace 从 Runner Invocation 读取用户和会话，并在本次运行中复用同一个 Sandbox。
func (s *toolSet) currentWorkspace(ctx context.Context) (*e2b.Sandbox, error) {
	invocation, ok := agent.InvocationFromContext(ctx)
	if !ok || invocation == nil || invocation.Session == nil {
		return nil, fmt.Errorf("sandbox tools require a Runner invocation with a session")
	}
	if value, ok := invocation.GetState(workspaceStateKey); ok {
		workspace, ok := value.(*e2b.Sandbox)
		if !ok {
			return nil, fmt.Errorf("sandbox invocation state has an unexpected value")
		}
		return workspace, nil
	}
	userID := strings.TrimSpace(invocation.Session.UserID)
	sessionID := strings.TrimSpace(invocation.Session.ID)
	workspace, err := s.workspaces.Workspace(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	invocation.SetState(workspaceStateKey, workspace)
	return workspace, nil
}
func toolDescription(action string) string { return action + "\n\n" + Workspace.ToolGuide() }
