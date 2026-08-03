package agentrun

import (
	"context"
	"strings"
	"testing"

	"edith/backend-v2/internal/skills"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"
)

func TestSkillInstructionUsesOnlySummary(t *testing.T) {
	got := skillInstruction([]skills.SkillSummary{
		{Name: "current-time", Description: "需要知道当前时间时，调用 get_current_time 工具。"},
	}, "")
	want := "可用公共 Skills：\n- current-time：需要知道当前时间时，调用 get_current_time 工具。\n完整 Skill 文件和资源位于 Sandbox 工作区：公共 Skills skills/system/<skill-name>/，用户 Skills skills/custom/<skill-name>/。需要完整规则或资源时，通过 Sandbox 文件工具读取对应目录。"
	if got != want {
		t.Fatalf("skillInstruction() = %q, want %q", got, want)
	}
}

func TestSkillInstructionIncludesUserOverview(t *testing.T) {
	got := skillInstruction(nil, "# 用户 Skills 总览\n- `daily-summary`：生成每日总结。")
	want := "可用用户 Skills：\n# 用户 Skills 总览\n- `daily-summary`：生成每日总结。\n完整 Skill 文件和资源位于 Sandbox 工作区：公共 Skills skills/system/<skill-name>/，用户 Skills skills/custom/<skill-name>/。需要完整规则或资源时，通过 Sandbox 文件工具读取对应目录。"
	if got != want {
		t.Fatalf("skillInstruction() = %q, want %q", got, want)
	}
}

func TestMessageWithUploadsKeepsAttachmentNamesInHistory(t *testing.T) {
	got := messageWithUploads("请处理这些文件", []string{"uploads/report.pdf", "uploads/data (2).csv"})
	want := "请处理这些文件\n\n附件：\n- `report.pdf`\n- `data (2).csv`"
	if got != want {
		t.Fatalf("messageWithUploads() = %q, want %q", got, want)
	}
}

func TestFrameworkRunOptionsInjectsSkillInstructionOnce(t *testing.T) {
	runOptions := agent.NewRunOptions(frameworkRunOptions(runOptionInput{
		requestID:         "request-1",
		modelID:           "model-1",
		contextWindow:     256_000,
		apiKey:            "key-1",
		globalInstruction: "global",
		instruction:       "skill instruction",
	})...)
	if runOptions.Instruction != "skill instruction" {
		t.Fatalf("RunOptions.Instruction = %q", runOptions.Instruction)
	}
	if runOptions.ModelContextWindow != 256_000 {
		t.Fatalf("RunOptions.ModelContextWindow = %d", runOptions.ModelContextWindow)
	}
}

type summaryTestModel struct {
	lastRequest *model.Request
}

func (m *summaryTestModel) GenerateContent(_ context.Context, request *model.Request) (<-chan *model.Response, error) {
	m.lastRequest = request
	responses := make(chan *model.Response, 1)
	responses <- &model.Response{Choices: []model.Choice{{
		Message: model.Message{Content: "summary"},
	}}}
	close(responses)
	return responses, nil
}

func (m *summaryTestModel) Info() model.Info {
	return model.Info{Name: "summary-test", ContextWindow: 10_000}
}

func TestSessionSummarizerUsesCurrentRunModelAndHeaders(t *testing.T) {
	baseModel := &summaryTestModel{}
	summarizer := NewSessionSummarizer()
	contextual, ok := summarizer.(summary.ContextAwareSummarizer)
	if !ok {
		t.Fatal("session summarizer is not context-aware")
	}

	invocation := agent.NewInvocation(
		agent.WithInvocationModel(baseModel),
		agent.WithInvocationRunOptions(agent.NewRunOptions(
			agent.WithModelContextWindow(10_000),
			agent.WithModelRequestHeaders(map[string]string{
				"Authorization": "Bearer user-key",
				"X-Request":     "from-invocation",
			}),
		)),
	)
	ctx := agent.NewInvocationContext(context.Background(), invocation)
	sess := &frameworksession.Session{Events: []event.Event{{
		Response: &model.Response{Choices: []model.Choice{{
			Message: model.Message{Content: strings.Repeat("word ", 5_000)},
		}}},
	}}}
	if !contextual.ShouldSummarizeWithContext(ctx, sess) {
		t.Fatal("summary should trigger at 40% of the current model context window")
	}

	if _, err := summarizer.Summarize(ctx, sess); err != nil {
		t.Fatalf("summarize() error = %v", err)
	}
	if baseModel.lastRequest == nil {
		t.Fatal("summary model was not called")
	}
	if got := baseModel.lastRequest.Headers["Authorization"]; got != "Bearer user-key" {
		t.Fatalf("summary Authorization header = %q", got)
	}
}
