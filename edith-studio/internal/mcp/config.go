package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const configFileName = "mcp.json"

// Load 读取用户级与项目级 mcp.json 并合并；项目级覆盖用户级同名 server。
// 配置文件不存在时返回空配置；存在但无法解析时返回错误。
func Load(projectRoot string) (Config, error) {
	merged := Config{Servers: make(map[string]ServerConfig)}
	userPath, err := userConfigPath()
	if err != nil {
		return Config{}, err
	}
	if err := loadFile(userPath, merged.Servers); err != nil {
		return Config{}, err
	}
	if err := loadFile(filepath.Join(projectRoot, ".edith", configFileName), merged.Servers); err != nil {
		return Config{}, err
	}
	return merged, nil
}

func userConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".edith", configFileName), nil
}

// loadFile 读取一个 mcp.json 并合并进 target；同名 server 由后读取的覆盖。
func loadFile(path string, target map[string]ServerConfig) error {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var config Config
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	for name, server := range config.Servers {
		target[name] = server
	}
	return nil
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
