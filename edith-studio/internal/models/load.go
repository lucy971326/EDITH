package models

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFileName = "models.yaml"

// Load 读取用户级 models.yaml，并创建启动期模型模块。
func Load() (*Module, error) {
	path, err := userConfigPath()
	if err != nil {
		return nil, err
	}
	config, err := readConfig(path)
	if err != nil {
		return nil, err
	}
	return Build(config)
}

func userConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".edith", configFileName), nil
}

func readConfig(path string) (Config, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, fmt.Errorf("%s does not exist; create it before starting EDITH Studio", path)
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return config, nil
}

func validate(config Config) error {
	if strings.TrimSpace(config.Default) == "" {
		return fmt.Errorf("models.yaml: default is required")
	}
	if len(config.Providers) == 0 {
		return fmt.Errorf("models.yaml: at least one provider is required")
	}
	if len(config.Models) == 0 {
		return fmt.Errorf("models.yaml: at least one model is required")
	}
	if _, ok := config.Models[config.Default]; !ok {
		return fmt.Errorf("models.yaml: default model %q is not configured", config.Default)
	}

	providerIDs := make([]string, 0, len(config.Providers))
	for providerID := range config.Providers {
		providerIDs = append(providerIDs, providerID)
	}
	sort.Strings(providerIDs)
	for _, providerID := range providerIDs {
		providerID = strings.TrimSpace(providerID)
		provider := config.Providers[providerID]
		if providerID == "" {
			return fmt.Errorf("models.yaml: provider ID is required")
		}
		if strings.TrimSpace(provider.APIKey) == "" {
			return fmt.Errorf("models.yaml: provider %q needs api_key", providerID)
		}
		if strings.TrimSpace(provider.BaseURL) == "" {
			return fmt.Errorf("models.yaml: provider %q needs base_url", providerID)
		}
	}

	modelIDs := make([]string, 0, len(config.Models))
	for modelID := range config.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		modelConfig := config.Models[modelID]
		if modelID == "" {
			return fmt.Errorf("models.yaml: model ID is required")
		}
		if strings.TrimSpace(modelConfig.Provider) == "" || strings.TrimSpace(modelConfig.Name) == "" {
			return fmt.Errorf("models.yaml: model %q needs provider and name", modelID)
		}
		if _, ok := config.Providers[modelConfig.Provider]; !ok {
			return fmt.Errorf("models.yaml: model %q references unknown provider %q", modelID, modelConfig.Provider)
		}
		if modelConfig.ContextWindow <= 0 {
			return fmt.Errorf("models.yaml: model %q needs a positive context_window", modelID)
		}
		if err := validateThinking(modelID, modelConfig.Thinking); err != nil {
			return err
		}
	}
	return nil
}

func validateThinking(modelID string, config ThinkingConfig) error {
	defaultMode := strings.TrimSpace(config.Default)
	if defaultMode == "" {
		return fmt.Errorf("models.yaml: model %q needs thinking.default", modelID)
	}
	if len(config.Modes) == 0 {
		return fmt.Errorf("models.yaml: model %q needs at least one thinking mode", modelID)
	}
	seen := make(map[string]struct{}, len(config.Modes))
	for _, mode := range config.Modes {
		mode = strings.TrimSpace(mode)
		if mode == "" {
			return fmt.Errorf("models.yaml: model %q has an empty thinking mode", modelID)
		}
		if _, exists := seen[mode]; exists {
			return fmt.Errorf("models.yaml: model %q repeats thinking mode %q", modelID, mode)
		}
		seen[mode] = struct{}{}
	}
	if _, ok := seen[defaultMode]; !ok {
		return fmt.Errorf("models.yaml: model %q default thinking mode %q is not listed in modes", modelID, defaultMode)
	}
	return nil
}
