package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/webhook/github", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("========== 收到 GitHub Webhook ==========")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("读取 body 失败: %v", err)
		}
		defer r.Body.Close()

		// 保存到文件
		os.MkdirAll("webhook_logs", 0755)
		filename := fmt.Sprintf("webhook_logs/%s.json", time.Now().Format("20060102_150405"))
		err = os.WriteFile(filename, body, 0644)
		if err != nil {
			log.Printf("保存文件失败: %v", err)
		} else {
			fmt.Printf("已保存到: %s\n", filename)
		}

		fmt.Printf("X-GitHub-Event: %s\n", r.Header.Get("X-GitHub-Event"))
		fmt.Println("==========================================")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	fmt.Println("服务器启动在 :2026")
	log.Fatal(http.ListenAndServe(":2026", nil))
}
