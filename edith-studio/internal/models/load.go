package models

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFileName = "models.yaml"

// Load 读取用户级 models.yaml。
func Load() (Config, error) {
	userConfigPath, err := userConfigPath()
	if err != nil {
		return Config{}, err
	}
	config, err := readConfig(userConfigPath)
	if err != nil {
		return Config{}, err
	}
	if err := validate(config); err != nil {
		return Config{}, err
	}
	return config, nil
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
	if len(config.Models) == 0 {
		return fmt.Errorf("models.yaml: at least one model is required")
	}
	if _, ok := config.Models[config.Default]; !ok {
		return fmt.Errorf("models.yaml: default model %q is not configured", config.Default)
	}
	for modelID, model := range config.Models {
		if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Name) == "" {
			return fmt.Errorf("models.yaml: model %q needs provider and name", modelID)
		}
		provider, ok := config.Providers[model.Provider]
		if !ok {
			return fmt.Errorf("models.yaml: model %q references unknown provider %q", modelID, model.Provider)
		}
		if strings.TrimSpace(provider.APIKey) == "" {
			return fmt.Errorf("models.yaml: provider %q needs api_key", model.Provider)
		}
		if strings.TrimSpace(provider.BaseURL) == "" {
			return fmt.Errorf("models.yaml: provider %q needs base_url", model.Provider)
		}
	}
	return nil
}
