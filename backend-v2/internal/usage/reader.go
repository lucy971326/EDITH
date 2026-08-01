package usage

import (
	"context"
	"strings"
)

// Reader 读取其他模块展示所需的用量数据。
type Reader struct {
	store *store
}

// Status 返回该用户指定运行的持久化状态。
func (r *Reader) Status(ctx context.Context, userID, requestID string) (string, error) {
	return r.store.status(ctx, strings.TrimSpace(userID), strings.TrimSpace(requestID))
}

// SessionSummary 返回一个会话的服务器计算用量汇总。
func (r *Reader) SessionSummary(ctx context.Context, userID, sessionID string) (Summary, error) {
	return r.store.sessionSummary(ctx, strings.TrimSpace(userID), strings.TrimSpace(sessionID))
}
