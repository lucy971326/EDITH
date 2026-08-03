package main

import (
	"log"
	"os"

	"edith/backend-v2/internal/images"

	"github.com/joho/godotenv"
)

// loadEnvironment 加载本地开发配置；进程环境变量优先。
// 兼容两种布局：backend-v2/.env（旧），或仓库根目录 .env（统一配置）。
// 在 backend-v2 下 go run 时 ../.env 即根目录；从根目录运行时 .env 即根目录。
func loadEnvironment() {
	for _, path := range []string{".env", "../.env"} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Fatalf("加载 %s: %v", path, err)
		}
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
