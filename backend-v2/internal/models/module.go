package models

import (
	"errors"

	"edith/backend-v2/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// Dependencies 是 models 运行所需的明确外部能力。
type Dependencies struct {
	Providers *userconfig.Providers
}

// Module 是模型目录模块对外公开的能力集合。
type Module struct {
	Catalog *Catalog
	HTTP    *HTTP
}

// Catalog 提供浏览器目录与 Runner 注册模型。
type Catalog struct {
	providers   []ProviderInfo
	definitions []Definition
}

// New 创建模型目录模块。Providers 仅供 HTTP 查询可用模型。
func New(deps Dependencies) (*Module, error) {
	if deps.Providers == nil {
		return nil, errors.New("models requires provider credentials")
	}
	catalog := &Catalog{providers: providerDefinitions(), definitions: modelDefinitions()}
	httpAPI := &HTTP{catalog: catalog, providers: deps.Providers}
	return &Module{Catalog: catalog, HTTP: httpAPI}, nil
}

// Providers 返回复制后的供应商目录，调用方可安全读取。
func (c *Catalog) Providers() []ProviderInfo {
	providers := make([]ProviderInfo, len(c.providers))
	copy(providers, c.providers)
	return providers
}

// Models 返回复制后的浏览器模型目录。
func (c *Catalog) Models() []Info {
	models := make([]Info, 0, len(c.definitions))
	for _, definition := range c.definitions {
		models = append(models, definition.Info)
	}
	return models
}

// Find 按 EDITH 稳定模型 ID 查找定义。
func (c *Catalog) Find(modelID string) (Definition, bool) {
	for _, definition := range c.definitions {
		if definition.ID == modelID {
			return definition, true
		}
	}
	return Definition{}, false
}

// Registered 返回 ManagedRunner 所需的模型 ID 到适配器映射。
func (c *Catalog) Registered() map[string]model.Model {
	registered := make(map[string]model.Model, len(c.definitions))
	for _, definition := range c.definitions {
		registered[definition.ID] = definition.Model
	}
	return registered
}
