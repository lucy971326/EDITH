package agentrun

import (
	"log"

	"edith/backend-v2/internal/agentstream"
	"edith/backend-v2/internal/usage"

	"trpc.group/trpc-go/trpc-agent-go/event"
)

// runLifecycle 统一管理一次已启动任务的收尾责任。
type runLifecycle struct {
	service    *Service
	configured *configuredRun
	finished   bool
}

// readFrameworkEvents 消费完整框架事件流，并在 channel 关闭前完成任务收尾。
func (s *Service) readFrameworkEvents(
	configured *configuredRun,
	rawEvents <-chan *event.Event,
	events chan<- agentstream.Event,
) {
	lifecycle := &runLifecycle{service: s, configured: configured}
	defer close(events)
	defer lifecycle.Close()

	request := configured.request
	assistantID := "assistant_" + request.RequestID
	decoder := agentstream.NewDecoder(assistantID)
	events <- agentstream.Event{
		Type:        "run.started",
		SessionID:   request.SessionID,
		RequestID:   request.RequestID,
		AssistantID: assistantID,
	}

	var tokens usage.Tokens
	for rawEvent := range rawEvents {
		decoded := decoder.DecodeFrameworkEvent(rawEvent)
		if decoded.ErrorMessage != "" && !s.userStops.contains(request.RequestID) {
			events <- agentstream.Event{
				Type:      "run.error",
				RequestID: request.RequestID,
				SessionID: request.SessionID,
				Error:     &agentstream.APIError{Type: "runner_error", Message: decoded.ErrorMessage},
			}
		}
		for _, neutralEvent := range decoded.Events {
			events <- neutralEvent
		}
		if decoded.Usage != nil {
			usage.AddTokens(&tokens, decoded.Usage, !configured.definition.DoesNotReportCachedPromptTokens)
		}
		if !decoded.Completed {
			continue
		}

		terminalEvent, err := lifecycle.Finish(tokens)
		if err != nil {
			events <- agentstream.Event{
				Type:      "run.error",
				RequestID: request.RequestID,
				SessionID: request.SessionID,
				Error:     &agentstream.APIError{Type: "internal_error", Message: err.Error()},
			}
			return
		}
		events <- terminalEvent
		return
	}
}

// Finish 写入用量并生成 completed 或 canceled 终止事件。
func (l *runLifecycle) Finish(tokens usage.Tokens) (agentstream.Event, error) {
	run := l.configured.run
	summary, err := l.service.usage.Finish(l.configured.ctx, run, tokens)
	if err != nil {
		return agentstream.Event{}, err
	}
	l.finished = true
	eventType := "run.completed"
	if l.service.userStops.take(run.RequestID) {
		eventType = "run.canceled"
	}
	return agentstream.Event{
		Type:      eventType,
		RequestID: run.RequestID,
		SessionID: run.SessionID,
		Usage: &agentstream.SessionUsage{
			TotalTokens:          summary.TotalTokens,
			CachedPromptTokens:   summary.CachedPromptTokens,
			UncachedPromptTokens: summary.UncachedPromptTokens,
			CompletionTokens:     summary.CompletionTokens,
			CacheHitRate:         summary.CacheHitRate,
		},
	}, nil
}

// Close 释放 MCP、会话 lane 和取消标记；异常结束时同时标记用量失败。
func (l *runLifecycle) Close() {
	run := l.configured.run
	if !l.finished {
		if err := l.service.usage.Fail(l.configured.ctx, run.RequestID); err != nil {
			log.Printf("标记任务 %q 失败: %v", run.RequestID, err)
		}
	}
	l.configured.Close()
	l.service.lanes.release(run.UserID, run.SessionID, run.RequestID)
	l.service.userStops.take(run.RequestID)
}
