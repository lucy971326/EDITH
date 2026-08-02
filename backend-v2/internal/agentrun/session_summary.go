package agentrun

import (
	"context"
	"errors"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"
)

const sessionSummaryContextRatio = 0.4

// NewSessionSummarizer 创建按本次 AgentRun 动态选择模型的会话摘要器。
// 摘要只在新增事件达到当前模型上下文窗口的 40% 时触发。
func NewSessionSummarizer() summary.SessionSummarizer {
	return summary.NewDynamicSummarizer(func(
		ctx context.Context,
		_ *session.Session,
	) (summary.SessionSummarizer, error) {
		invocation, ok := agent.InvocationFromContext(ctx)
		if !ok || invocation == nil || invocation.Model == nil {
			return nil, nil
		}
		return summary.NewSummarizer(
			&invocationSummaryModel{base: invocation.Model},
			summary.WithContextThreshold(
				summary.WithContextThresholdRatio(sessionSummaryContextRatio),
			),
		), nil
	})
}

// invocationSummaryModel 让框架摘要请求复用本次运行的请求头。
// API Key 只从 Invocation 读取，不进入 Session 或摘要文本。
type invocationSummaryModel struct {
	base model.Model
}

func (m *invocationSummaryModel) GenerateContent(
	ctx context.Context,
	request *model.Request,
) (<-chan *model.Response, error) {
	if m == nil || m.base == nil {
		return nil, errors.New("summary model is not configured")
	}
	if request == nil {
		return nil, errors.New("summary request is nil")
	}

	requestCopy := *request
	if invocation, ok := agent.InvocationFromContext(ctx); ok && invocation != nil {
		if len(invocation.RunOptions.ModelRequestHeaders) > 0 {
			requestCopy.Headers = cloneRequestHeaders(request.Headers)
			for key, value := range invocation.RunOptions.ModelRequestHeaders {
				if _, exists := requestCopy.Headers[key]; !exists {
					requestCopy.Headers[key] = value
				}
			}
		}
	}
	return m.base.GenerateContent(ctx, &requestCopy)
}

func (m *invocationSummaryModel) Info() model.Info {
	if m == nil || m.base == nil {
		return model.Info{}
	}
	return m.base.Info()
}

func cloneRequestHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return make(map[string]string)
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}
