package main

import (
	"errors"
	"log"
	"os"

	"edith/backend-v2/internal/images"

	"github.com/joho/godotenv"
)

// loadEnvironment 加载本地开发配置；进程环境变量优先。
func loadEnvironment() {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("加载 .env: %v", err)
	}
}

func databasePath() string {
	if path := os.Getenv("EDITH_DB_PATH"); path != "" {
		return path
	}
	return "edith-v2.db"
}

func runtimeAddress() string {
	if address := os.Getenv("EDITH_ADDR"); address != "" {
		return address
	}
	return "127.0.0.1:8080"
}

func sandboxTemplate() string {
	template := os.Getenv("EDITH_E2B_TEMPLATE")
	if template == "" {
		log.Fatal("EDITH_E2B_TEMPLATE 是必填配置")
	}
	return template
}

func imageConfig() images.Config {
	return images.Config{
		Bucket: os.Getenv("EDITH_COS_BUCKET"), Region: os.Getenv("EDITH_COS_REGION"),
		SecretID: os.Getenv("EDITH_COS_SECRET_ID"), SecretKey: os.Getenv("EDITH_COS_SECRET_KEY"),
	}
}
