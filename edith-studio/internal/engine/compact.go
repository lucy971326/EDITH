package engine

import (
	"context"
	"fmt"
	"strings"

	"edith/studio/internal/models"
	"edith/studio/internal/session"
)

const compactOperationID = "command:/compact"

// Compact 为指定 Session 同步生成摘要，并把结果保存到会话存储。
// 摘要模型由本次输入的 ModelID 和 ThinkingMode 决定。
func (e *Engine) Compact(ctx context.Context, input CompactInput) error {
	input.SessionID = strings.TrimSpace(input.SessionID)
	if input.SessionID == "" {
		return ErrInvalidCompactInput
	}
	selection := models.Selection{
		ModelID:      strings.TrimSpace(input.ModelID),
		ThinkingMode: strings.TrimSpace(input.ThinkingMode),
	}
	if _, err := e.models.SummaryModel(selection); err != nil {
		return fmt.Errorf("resolve summary model: %w", err)
	}
	if err := e.reserveSession(input.SessionID, compactOperationID); err != nil {
		return err
	}
	defer e.releaseSession(input.SessionID)

	key, err := session.SessionKey(e.workspaceID, input.SessionID)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	storedSession, err := e.sessions.GetSession(ctx, key)
	if err != nil {
		return fmt.Errorf("get session for compaction: %w", err)
	}
	if storedSession == nil {
		return session.ErrSessionNotFound
	}

	ctx = models.WithSelection(ctx, selection)
	if err := e.sessions.CreateSessionSummary(
		ctx,
		storedSession,
		e.filterKey,
		true,
	); err != nil {
		return fmt.Errorf("create session summary: %w", err)
	}
	return nil
}
