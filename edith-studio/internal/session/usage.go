package session

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/event"
)

// ContextUsage 返回该会话最近一次模型请求的官方 prompt token 用量。
// 只信厂商上报：遍历会话事件，取最后一条非零 Usage.PromptTokens；
// 没有官方数据时返回 0，不做任何本地估算。
func (m *Module) ContextUsage(ctx context.Context, workspaceID, sessionID string) (ContextUsage, error) {
	key, err := sessionKey(workspaceID, sessionID)
	if err != nil {
		return ContextUsage{}, err
	}
	storedSession, err := m.service.GetSession(ctx, key)
	if err != nil {
		return ContextUsage{}, fmt.Errorf("get session: %w", err)
	}
	if storedSession == nil {
		return ContextUsage{}, ErrSessionNotFound
	}
	return ContextUsage{PromptTokens: lastPromptTokens(storedSession.GetEvents())}, nil
}

func lastPromptTokens(events []event.Event) int {
	promptTokens := 0
	for _, frameworkEvent := range events {
		if frameworkEvent.Response == nil || frameworkEvent.Response.Usage == nil {
			continue
		}
		if frameworkEvent.Response.Usage.PromptTokens > 0 {
			promptTokens = frameworkEvent.Response.Usage.PromptTokens
		}
	}
	return promptTokens
}
