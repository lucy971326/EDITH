package main

import (
	"log"
	"os"
	"strings"
)

const systemPromptPath = "prompts/edith.md"

// loadSystemPrompt 读取独立的系统提示词文件，便于不改 Go 代码就调整 Agent 行为。
func loadSystemPrompt() string {
	content, err := os.ReadFile(systemPromptPath)
	if err != nil {
		log.Fatalf("read system prompt %q: %v", systemPromptPath, err)
	}

	prompt := strings.TrimSpace(string(content))
	if prompt == "" {
		log.Fatalf("system prompt %q is empty", systemPromptPath)
	}
	return prompt
}
