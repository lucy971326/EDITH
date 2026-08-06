package models

import (
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// Registry 保存启动时创建的默认模型和全部模型实例。
type Registry struct {
	Default model.Model
	All     map[string]model.Model
}

// Open 读取用户配置并创建 OpenAI 兼容模型实例。
func Open() (Registry, error) {
	config, err := Load()
	if err != nil {
		return Registry{}, err
	}
	registry := Registry{All: make(map[string]model.Model, len(config.Models))}
	for modelID, modelConfig := range config.Models {
		provider := config.Providers[modelConfig.Provider]
		registry.All[modelID] = openai.New(
			modelConfig.Name,
			openai.WithAPIKey(provider.APIKey),
			openai.WithBaseURL(provider.BaseURL),
		)
	}
	registry.Default = registry.All[config.Default]
	if registry.Default == nil {
		return Registry{}, fmt.Errorf("default model %q was not created", config.Default)
	}
	return registry, nil
}
