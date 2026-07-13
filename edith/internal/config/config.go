// Package config 从环境变量加载 EDITH 后端配置。
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	HTTPAddr           string
	EdithDBPath        string
	SessionDBPath      string
	GitHubAppID        string
	GitHubPrivateKeyPath string
	GitHubWebhookSecret  string
	ClerkSecretKey       string
	ClerkAuthorizedParties string
	E2BAPIKey     string
	E2BTemplateID string
	LLMModel      string
	LLMBaseURL    string
	LLMAPIKey     string
}

// Load 加载配置；HTTP_ADDR 是唯一强制项。
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),

		EdithDBPath:   getenv("EDITH_DB_PATH", "data/edith.db"),
		SessionDBPath: getenv("SESSION_DB_PATH", "data/sessions.db"),

		GitHubAppID:          os.Getenv("GITHUB_APP_ID"),
		GitHubPrivateKeyPath: os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
		GitHubWebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),

		ClerkSecretKey:         os.Getenv("CLERK_SECRET_KEY"),
		ClerkAuthorizedParties: os.Getenv("CLERK_AUTHORIZED_PARTIES"),

		E2BAPIKey:     os.Getenv("E2B_API_KEY"),
		E2BTemplateID: os.Getenv("E2B_TEMPLATE_ID"),

		LLMModel:   os.Getenv("LLM_MODEL"),
		LLMBaseURL: os.Getenv("LLM_BASE_URL"),
		LLMAPIKey:  os.Getenv("LLM_API_KEY"),
	}

	if cfg.HTTPAddr == "" {
		return nil, errors.New("config: HTTP_ADDR is required")
	}

	cfg.EdithDBPath = toAbs(cfg.EdithDBPath)
	cfg.SessionDBPath = toAbs(cfg.SessionDBPath)
	return cfg, nil
}

// GitHubAppIDInt 把 App ID 字符串解析为 int64。
func (c *Config) GitHubAppIDInt() int64 {
	if c.GitHubAppID == "" {
		return 0
	}
	v, err := strconv.ParseInt(c.GitHubAppID, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func toAbs(p string) string {
	if p == "" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}