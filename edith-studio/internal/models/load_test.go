package models

import "testing"

func TestValidateRejectsUnknownProvider(t *testing.T) {
	err := validate(Config{
		Default: "model-a",
		Models: map[string]ModelConfig{
			"model-a": {Provider: "missing", Name: "a"},
		},
	})
	if err == nil {
		t.Fatal("validate accepted a missing provider")
	}
}

func TestValidateRequiresProviderBaseURL(t *testing.T) {
	err := validate(Config{
		Default: "model-a",
		Providers: map[string]ProviderConfig{
			"deepseek": {APIKey: "secret"},
		},
		Models: map[string]ModelConfig{
			"model-a": {Provider: "deepseek", Name: "deepseek-chat"},
		},
	})
	if err == nil {
		t.Fatal("validate accepted a provider without base_url")
	}
}
