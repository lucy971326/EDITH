package models

import (
	"fmt"
	"sort"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// Module 是启动期创建的模型目录和模型能力组装对象。
type Module struct {
	// defaultID 是用户配置的稳定默认模型身份。
	defaultID string
	// models 是启动时创建、供 Agent 调用的模型实例能力组。
	models map[string]model.Model
	// entries 是模型实例与 Web Profile 的对应关系。
	entries map[string]modelEntry
}

// modelEntry 把一个模型实例的供应商协议和公开 Profile 放在一起。
type modelEntry struct {
	// profile 是不含密钥的公开模型能力描述。
	profile Profile
	// provider 是用于选择请求字段格式的供应商身份。
	provider string
}

// Build 校验配置并创建所有长期模型实例。
func Build(config Config) (*Module, error) {
	if err := validate(config); err != nil {
		return nil, err
	}

	modelIDs := make([]string, 0, len(config.Models))
	for modelID := range config.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	module := &Module{
		defaultID: config.Default,
		models:    make(map[string]model.Model, len(modelIDs)),
		entries:   make(map[string]modelEntry, len(modelIDs)),
	}
	for _, modelID := range modelIDs {
		modelConfig := config.Models[modelID]
		providerConfig := config.Providers[modelConfig.Provider]
		variant, err := providerVariant(modelConfig.Provider, providerConfig)
		if err != nil {
			return nil, fmt.Errorf("models.yaml: model %q: %w", modelID, err)
		}

		modelInstance := openai.New(
			modelConfig.Name,
			openai.WithAPIKey(providerConfig.APIKey),
			openai.WithBaseURL(providerConfig.BaseURL),
			openai.WithVariant(variant),
			openai.WithContextWindow(modelConfig.ContextWindow),
		)
		profile := Profile{
			ID:            modelID,
			Provider:      modelConfig.Provider,
			Name:          modelConfig.Name,
			ContextWindow: modelConfig.ContextWindow,
			Vision:        modelConfig.Vision,
			Thinking: ThinkingProfile{
				DefaultMode: modelConfig.Thinking.Default,
				Modes:       append([]string(nil), modelConfig.Thinking.Modes...),
			},
		}
		module.models[modelID] = modelInstance
		module.entries[modelID] = modelEntry{
			profile:  profile,
			provider: strings.ToLower(strings.TrimSpace(modelConfig.Provider)),
		}
	}
	return module, nil
}

// AgentModels 返回供 Workspace 注册到 LLMAgent 的模型实例。
func (m *Module) AgentModels() map[string]model.Model {
	instances := make(map[string]model.Model, len(m.models))
	for modelID, modelInstance := range m.models {
		instances[modelID] = modelInstance
	}
	return instances
}

// DefaultModel 返回启动时配置的默认模型实例。
func (m *Module) DefaultModel() model.Model {
	return m.models[m.defaultID]
}

// Catalog 返回供 Studio 和 Web 使用的公开模型目录。
func (m *Module) Catalog() Catalog {
	modelIDs := make([]string, 0, len(m.entries))
	for modelID := range m.entries {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	profiles := make([]Profile, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		entry := m.entries[modelID]
		profile := entry.profile
		profile.Thinking.Modes = append([]string(nil), profile.Thinking.Modes...)
		profiles = append(profiles, profile)
	}
	return Catalog{DefaultModelID: m.defaultID, Models: profiles}
}

// RunOptions 把一次产品级模型选择转换为框架 RunOptions。
func (m *Module) RunOptions(selection Selection) ([]agent.RunOption, error) {
	modelID, entry, thinkingMode, err := m.resolveSelection(selection)
	if err != nil {
		return nil, err
	}

	options := []agent.RunOption{
		agent.WithModelName(modelID),
		agent.WithModelContextWindow(entry.profile.ContextWindow),
	}
	fields := thinkingFields(entry, thinkingMode)
	if len(fields) > 0 {
		options = append(options, agent.WithModelRequestExtraFields(fields))
	}
	return options, nil
}

func (m *Module) resolveSelection(selection Selection) (string, modelEntry, string, error) {
	modelID := strings.TrimSpace(selection.ModelID)
	if modelID == "" {
		modelID = m.defaultID
	}
	entry, ok := m.entries[modelID]
	if !ok {
		return "", modelEntry{}, "", fmt.Errorf("%w: %s", ErrUnknownModel, modelID)
	}

	thinkingMode := strings.TrimSpace(selection.ThinkingMode)
	if thinkingMode == "" {
		thinkingMode = entry.profile.Thinking.DefaultMode
	}
	if !contains(entry.profile.Thinking.Modes, thinkingMode) {
		return "", modelEntry{}, "", fmt.Errorf("%w: model %q does not support %q", ErrUnsupportedThinkingMode, modelID, thinkingMode)
	}
	return modelID, entry, thinkingMode, nil
}

func providerVariant(providerID string, config ProviderConfig) (openai.Variant, error) {
	variantName := strings.ToLower(strings.TrimSpace(config.Variant))
	if variantName == "" {
		variantName = strings.ToLower(strings.TrimSpace(providerID))
	}
	switch variantName {
	case "openai":
		return openai.VariantOpenAI, nil
	case "deepseek":
		return openai.VariantDeepSeek, nil
	case "qwen":
		return openai.VariantQwen, nil
	case "hunyuan":
		return openai.VariantHunyuan, nil
	case "glm":
		return openai.VariantGLM, nil
	case "minimax":
		return openai.VariantMiniMax, nil
	case "kimi":
		return openai.VariantKimi, nil
	case "mimo":
		// MiMo 使用 OpenAI 兼容请求；它的 thinking 字段由本模块补充。
		return openai.VariantOpenAI, nil
	default:
		return "", fmt.Errorf("unsupported provider variant %q", variantName)
	}
}

func thinkingFields(entry modelEntry, mode string) map[string]any {
	enabled := thinkingEnabled(entry.provider, mode)
	thinkingType := "disabled"
	if enabled {
		thinkingType = "enabled"
	}

	switch entry.provider {
	case "deepseek", "glm", "hunyuan":
		fields := map[string]any{"thinking": map[string]string{"type": thinkingType}}
		if enabled && (entry.provider == "deepseek" || entry.provider == "glm") {
			fields["reasoning_effort"] = mode
		}
		return fields
	case "minimax":
		// MiniMax M3 用 reasoning_split 把思考内容放到 reasoning_details，
		// 这样 Engine 才能把它作为 reasoning.delta 单独展示。
		fields := map[string]any{"reasoning_split": true}
		if enabled {
			fields["thinking"] = map[string]string{"type": "adaptive"}
			return fields
		}
		fields["thinking"] = map[string]string{"type": "disabled"}
		return fields
	case "kimi":
		return map[string]any{"reasoning_effort": mode}
	case "qwen":
		return map[string]any{"enable_thinking": enabled}
	case "mimo":
		return map[string]any{"thinking": map[string]string{"type": thinkingType}}
	default:
		return map[string]any{"thinking": enabled}
	}
}

func thinkingEnabled(provider, mode string) bool {
	if strings.EqualFold(mode, "off") || strings.EqualFold(mode, "disabled") {
		return false
	}
	// GLM 将 none/minimal 定义为关闭思考，其余模式才是不同深度。
	if provider == "glm" && (strings.EqualFold(mode, "none") || strings.EqualFold(mode, "minimal")) {
		return false
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
