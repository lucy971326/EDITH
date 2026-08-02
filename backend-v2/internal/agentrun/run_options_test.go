package agentrun

import (
	"testing"

	"edith/backend-v2/internal/skills"

	"trpc.group/trpc-go/trpc-agent-go/agent"
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

func TestFrameworkRunOptionsInjectsSkillInstructionOnce(t *testing.T) {
	runOptions := agent.NewRunOptions(frameworkRunOptions(runOptionInput{
		requestID:         "request-1",
		modelID:           "model-1",
		apiKey:            "key-1",
		globalInstruction: "global",
		instruction:       "skill instruction",
	})...)
	if runOptions.Instruction != "skill instruction" {
		t.Fatalf("RunOptions.Instruction = %q", runOptions.Instruction)
	}
}
