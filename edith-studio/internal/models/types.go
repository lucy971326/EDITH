// Package models 管理用户模型配置、模型实例和模型能力。
package models

import "errors"

var (
	// ErrUnknownModel 表示请求选择了没有注册的模型。
	ErrUnknownModel = errors.New("unknown model")
	// ErrUnsupportedThinkingMode 表示当前模型不支持请求的思考模式。
	ErrUnsupportedThinkingMode = errors.New("unsupported thinking mode")
)

// Config 是用户级 models.yaml 的完整格式。
type Config struct {
	// Default 是启动后默认使用的模型 ID。
	Default string `yaml:"default"`
	// Providers 保存供应商连接信息；API Key 只在后端内存中使用。
	Providers map[string]ProviderConfig `yaml:"providers"`
	// Models 保存模型 ID 到真实模型配置的映射。
	Models map[string]ModelConfig `yaml:"models"`
}

// ProviderConfig 是一个 OpenAI 兼容供应商的连接配置。
type ProviderConfig struct {
	// APIKey 是调用供应商 API 的密钥，不会出现在 Web 返回值中。
	APIKey string `yaml:"api_key"`
	// BaseURL 是供应商 OpenAI 兼容 API 的入口地址。
	BaseURL string `yaml:"base_url"`
	// Variant 是框架的供应商协议变体；为空时使用供应商 ID。
	Variant string `yaml:"variant,omitempty"`
}

// ModelConfig 是一个模型实例的配置和对外能力声明。
type ModelConfig struct {
	// Provider 是 Providers 中的供应商 ID。
	Provider string `yaml:"provider"`
	// Name 是供应商真正识别的模型名称。
	Name string `yaml:"name"`
	// ContextWindow 是模型上下文窗口大小，单位为 token。
	ContextWindow int `yaml:"context_window"`
	// Vision 表示模型是否支持视觉输入。
	Vision bool `yaml:"vision"`
	// Thinking 声明前端可选择的思考模式。
	Thinking ThinkingConfig `yaml:"thinking"`
}

// ThinkingConfig 是 models.yaml 中的思考模式配置。
type ThinkingConfig struct {
	// Default 是该模型未指定思考模式时使用的模式 ID。
	Default string `yaml:"default"`
	// Modes 是该模型允许前端选择的模式 ID 列表。
	Modes []string `yaml:"modes"`
}

// ThinkingProfile 是返回给 Web 的思考能力描述。
type ThinkingProfile struct {
	// DefaultMode 是该模型的默认思考模式。
	DefaultMode string `json:"defaultMode"`
	// Modes 是当前模型允许选择的思考模式。
	Modes []string `json:"modes"`
}

// Profile 是后端向 Web 暴露的单个模型能力，不包含 API Key。
type Profile struct {
	// ID 是 EDITH 内部使用的稳定模型选择 ID。
	ID string `json:"id"`
	// Provider 是模型所属的供应商 ID。
	Provider string `json:"provider"`
	// Name 是供应商真正识别的模型名称。
	Name string `json:"name"`
	// ContextWindow 是模型上下文窗口大小，单位为 token。
	ContextWindow int `json:"contextWindow"`
	// Vision 表示模型是否支持视觉输入。
	Vision bool `json:"vision"`
	// Thinking 是模型的思考模式能力。
	Thinking ThinkingProfile `json:"thinking"`
}

// Catalog 是 Studio 返回给 Web 的完整模型目录。
type Catalog struct {
	// DefaultModelID 是后端默认模型的 ID。
	DefaultModelID string `json:"defaultModelId"`
	// Models 是所有启动时已注册的公开模型 Profile。
	Models []Profile `json:"models"`
}

// Selection 是一次 Run 对模型能力的选择。
type Selection struct {
	// ModelID 是本次 Run 使用的模型 ID；为空时使用模块默认模型。
	ModelID string
	// ThinkingMode 是本次 Run 的思考模式；为空时使用模型默认模式。
	ThinkingMode string
}
