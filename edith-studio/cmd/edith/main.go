package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"edith/studio/internal/studio"
)

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if err := loadDotEnv(filepath.Join(projectRoot, ".env")); err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := studio.Start(ctx, projectRoot, "127.0.0.1:8765"); err != nil {
		log.Fatal(err)
	}
}

// loadDotEnv 读取项目目录下的 .env；已有系统环境变量优先级更高。
func loadDotEnv(path string) error {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}

	for lineNumber, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("解析 %s 第 %d 行失败，需要 KEY=VALUE 格式", path, lineNumber+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value, err = strconv.Unquote(value)
			if err != nil {
				return fmt.Errorf("解析 %s 第 %d 行失败: %w", path, lineNumber+1, err)
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("设置环境变量 %s 失败: %w", key, err)
			}
		}
	}
	return nil
}
