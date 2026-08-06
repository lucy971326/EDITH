package models

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/agent"
)

func TestBuildCatalogAndRunOptions(t *testing.T) {
	module, err := Build(testConfig())
	if err != nil {
		t.Fatalf("build module: %v", err)
	}

	catalog := module.Catalog()
	if catalog.DefaultModelID != "deepseek-pro" || len(catalog.Models) != 2 {
		t.Fatalf("catalog = %#v", catalog)
	}
	if catalog.Models[0].ID != "deepseek-flash" || catalog.Models[1].ID != "deepseek-pro" {
		t.Fatalf("catalog order = %#v", catalog.Models)
	}

	options, err := module.RunOptions(Selection{ModelID: "deepseek-pro", ThinkingMode: "high"})
	if err != nil {
		t.Fatalf("run options: %v", err)
	}
	var runOptions agent.RunOptions
	for _, option := range options {
		option(&runOptions)
	}
	if runOptions.ModelName != "deepseek-pro" || runOptions.ModelContextWindow != 1_000_000 {
		t.Fatalf("run options = %#v", runOptions)
	}
	if got := runOptions.ModelRequestExtraFields["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning effort = %#v", got)
	}
	thinking, ok := runOptions.ModelRequestExtraFields["thinking"].(map[string]string)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking fields = %#v", runOptions.ModelRequestExtraFields["thinking"])
	}
}

func TestRunOptionsRejectsUnknownSelection(t *testing.T) {
	module, err := Build(testConfig())
	if err != nil {
		t.Fatalf("build module: %v", err)
	}

	_, err = module.RunOptions(Selection{ModelID: "missing", ThinkingMode: "high"})
	if !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("error = %v, want ErrUnknownModel", err)
	}

	_, err = module.RunOptions(Selection{ModelID: "deepseek-pro", ThinkingMode: "low"})
	if !errors.Is(err, ErrUnsupportedThinkingMode) {
		t.Fatalf("error = %v, want ErrUnsupportedThinkingMode", err)
	}
}

func TestThinkingFieldsFollowProviderContract(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		mode     string
		want     map[string]any
	}{
		{
			name:     "glm disabled aliases",
			provider: "glm",
			mode:     "minimal",
			want:     map[string]any{"thinking": map[string]string{"type": "disabled"}},
		},
		{
			name:     "minimax adaptive",
			provider: "minimax",
			mode:     "enabled",
			want: map[string]any{
				"reasoning_split": true,
				"thinking":        map[string]string{"type": "adaptive"},
			},
		},
		{
			name:     "mimo disabled",
			provider: "mimo",
			mode:     "disabled",
			want:     map[string]any{"thinking": map[string]string{"type": "disabled"}},
		},
		{
			name:     "kimi reasoning effort",
			provider: "kimi",
			mode:     "high",
			want:     map[string]any{"reasoning_effort": "high"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := thinkingFields(modelEntry{provider: test.provider}, test.mode)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("fields = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCatalogDoesNotExposeAPIKey(t *testing.T) {
	module, err := Build(testConfig())
	if err != nil {
		t.Fatalf("build module: %v", err)
	}
	contents, err := json.Marshal(module.Catalog())
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	if string(contents) == "" || string(contents) == "{}" {
		t.Fatalf("catalog JSON = %s", contents)
	}
	if strings.Contains(string(contents), "secret") {
		t.Fatalf("catalog leaked API key: %s", contents)
	}
}

func testConfig() Config {
	return Config{
		Default: "deepseek-pro",
		Providers: map[string]ProviderConfig{
			"deepseek": {
				APIKey:  "secret",
				BaseURL: "https://api.deepseek.com",
				Variant: "deepseek",
			},
		},
		Models: map[string]ModelConfig{
			"deepseek-flash": {
				Provider:      "deepseek",
				Name:          "deepseek-v4-flash",
				ContextWindow: 1_000_000,
				Thinking:      ThinkingConfig{Default: "high", Modes: []string{"off", "high", "max"}},
			},
			"deepseek-pro": {
				Provider:      "deepseek",
				Name:          "deepseek-v4-pro",
				ContextWindow: 1_000_000,
				Thinking:      ThinkingConfig{Default: "max", Modes: []string{"off", "high", "max"}},
			},
		},
	}
}
