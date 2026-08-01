package usage

import (
	"context"
	"errors"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	runningStatus   = "running"
	completedStatus = "completed"
	failedStatus    = "failed"
)

var (
	// ErrRunAlreadyExists 表示同一个 requestID 已被记录。
	ErrRunAlreadyExists = errors.New("agent run already exists")
	// ErrRunNotFound 表示找不到指定的运行记录。
	ErrRunNotFound = errors.New("agent run not found")
)

// Recorder 在 AgentRun 接受、完成或失败时写入用量记录。
type Recorder struct {
	store *store
}

// Start 记录一个已接受的运行；重复 requestID 返回 ErrRunAlreadyExists。
func (r *Recorder) Start(ctx context.Context, run Run) error {
	return r.store.start(ctx, normalizeRun(run))
}

// Finish 写入完成运行的用量，并返回该会话新的用量汇总。
func (r *Recorder) Finish(ctx context.Context, run Run, tokens Tokens) (Summary, error) {
	run = normalizeRun(run)
	if err := r.store.finish(ctx, run, tokens); err != nil {
		return Summary{}, err
	}
	return (&Reader{store: r.store}).SessionSummary(ctx, run.UserID, run.SessionID)
}

// Fail 标记未完成运行失败；失败运行不计入会话用量。
func (r *Recorder) Fail(ctx context.Context, requestID string) error {
	return r.store.fail(ctx, strings.TrimSpace(requestID))
}

// AddTokens 累加一个完整模型响应的用量；流式增量不能传入此函数。
func AddTokens(tokens *Tokens, source *model.Usage, reportsCachedPromptTokens bool) {
	if source == nil {
		return
	}
	tokens.PromptTokens += source.PromptTokens
	tokens.CompletionTokens += source.CompletionTokens
	tokens.TotalTokens += source.TotalTokens
	if !reportsCachedPromptTokens {
		return
	}
	if tokens.CachedPromptTokens == nil {
		tokens.CachedPromptTokens = new(int)
	}
	*tokens.CachedPromptTokens += int(source.PromptTokensDetails.CachedTokens)
}

func normalizeRun(run Run) Run {
	run.RequestID = strings.TrimSpace(run.RequestID)
	run.UserID = strings.TrimSpace(run.UserID)
	run.SessionID = strings.TrimSpace(run.SessionID)
	run.ModelID = strings.TrimSpace(run.ModelID)
	return run
}
