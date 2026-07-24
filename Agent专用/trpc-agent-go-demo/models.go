package main

import (
	"fmt"
	"os"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// ModelInfo 是前端选择模型时需要展示的信息。
type ModelInfo struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Vision bool   `json:"vision"`
}

// loadedModels 是 EDITH 启动后可用的完整模型集合。
type loadedModels struct {
	clients   map[string]model.Model
	infos     []ModelInfo
	defaultID string
}

// modelDefinition 是一个模型的唯一登记处：Client 和展示能力始终一起定义。
type modelDefinition struct {
	info   ModelInfo
	client model.Model
}

// loadModels 读取环境变量，并创建 EDITH 当前可用的模型。
func loadModels() (loadedModels, error) {
	deepseekKey, err := requiredEnv("DEEPSEEK_API_KEY")
	if err != nil {
		return loadedModels{}, err
	}
	minimaxKey, err := requiredEnv("MINIMAX_API_KEY")
	if err != nil {
		return loadedModels{}, err
	}

	definitions := []modelDefinition{
		// 默认模型放第一项，前端会把列表第一项作为默认选择。
		{
			info: ModelInfo{ID: "MiniMax-M3", Label: "MiniMax M3", Vision: true},
			client: openai.New("MiniMax-M3",
				openai.WithAPIKey(minimaxKey),
				openai.WithBaseURL(envOr("MINIMAX_BASE_URL", "https://api.minimaxi.com/v1")),
				openai.WithExtraFields(map[string]any{"reasoning_split": true}),
			),
		},
		{
			info: ModelInfo{ID: "deepseek-v4-flash", Label: "DeepSeek V4 Flash"},
			client: openai.New("deepseek-v4-flash",
				openai.WithAPIKey(deepseekKey),
				openai.WithBaseURL(envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")),
			),
		},
		{
			info: ModelInfo{ID: "deepseek-v4-pro", Label: "DeepSeek V4 Pro"},
			client: openai.New("deepseek-v4-pro",
				openai.WithAPIKey(deepseekKey),
				openai.WithBaseURL(envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")),
			),
		},
	}

	models := loadedModels{
		clients:   make(map[string]model.Model, len(definitions)),
		infos:     make([]ModelInfo, 0, len(definitions)),
		defaultID: "MiniMax-M3",
	}
	for _, item := range definitions {
		if _, exists := models.clients[item.info.ID]; exists {
			return loadedModels{}, fmt.Errorf("duplicate model ID: %s", item.info.ID)
		}
		models.clients[item.info.ID] = item.client
		models.infos = append(models.infos, item.info)
	}
	if _, exists := models.clients[models.defaultID]; !exists {
		return loadedModels{}, fmt.Errorf("default model not found: %s", models.defaultID)
	}

	return models, nil
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}
