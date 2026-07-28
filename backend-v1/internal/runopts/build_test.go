package runopts

import (
	"testing"

	"edith/backend-v1/internal/tools"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestBuild(t *testing.T) {
	config := Config{
		RequestID:         "request-1",
		Stream:            true,
		ModelName:         "deepseek",
		APIKey:            "user-api-key",
		GlobalInstruction: "你是 EDITH。",
		Instruction:       "你可以使用默认工具。",
		AdditionalTools:   []tool.Tool{tools.GetCurrentTime},
	}

	options := Build(config)
	if got, want := len(options), 7; got != want {
		t.Fatalf("len(Build()) = %d, want %d", got, want)
	}

	var got agent.RunOptions
	for _, apply := range options {
		apply(&got)
	}

	if got.RequestID != config.RequestID {
		t.Errorf("RequestID = %q, want %q", got.RequestID, config.RequestID)
	}
	if got.Stream == nil || *got.Stream != config.Stream {
		t.Errorf("Stream = %v, want %t", got.Stream, config.Stream)
	}
	if got.ModelName != config.ModelName {
		t.Errorf("ModelName = %q, want %q", got.ModelName, config.ModelName)
	}
	if got.ModelRequestHeaders["Authorization"] != "Bearer "+config.APIKey {
		t.Errorf("Authorization header = %q", got.ModelRequestHeaders["Authorization"])
	}
	if got.GlobalInstruction != config.GlobalInstruction {
		t.Errorf("GlobalInstruction = %q, want %q", got.GlobalInstruction, config.GlobalInstruction)
	}
	if got.Instruction != config.Instruction {
		t.Errorf("Instruction = %q, want %q", got.Instruction, config.Instruction)
	}
	if got.AdditionalTools[0] != tools.GetCurrentTime {
		t.Error("AdditionalTools does not preserve the supplied tool")
	}
}
