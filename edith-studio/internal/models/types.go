// Package models 管理 models.yaml 及其对应的模型实例。
package models

// Config 是当前版本 models.yaml 的完整格式。
type Config struct {
	Default   string                    `yaml:"default"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Models    map[string]ModelConfig    `yaml:"models"`
}

// ProviderConfig 表示一个 OpenAI 兼容供应商。
type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// ModelConfig 将 Studio 模型名称关联到供应商的真实模型。
type ModelConfig struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
}
