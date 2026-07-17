package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

//go:embed web/*
var webFiles embed.FS

type UIPage struct {
	Title      string        `json:"title"`
	Background string        `json:"background"`
	Accent     string        `json:"accent"`
	Components []UIComponent `json:"components"`
}

type UIComponent struct {
	Type     string `json:"type" jsonschema:"enum=hero,enum=text,enum=card,enum=stat,enum=button"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Value    string `json:"value"`
	Label    string `json:"label"`
	Emphasis string `json:"emphasis" jsonschema:"enum=normal,enum=strong"`
}

type generateRequest struct {
	Prompt string `json:"prompt"`
}

type server struct {
	runner runner.Runner
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("请先设置 OPENAI_API_KEY")
	}

	llm := openai.New(
		"MiniMax-M3",
		openai.WithBaseURL("https://api.minimaxi.com/v1"),
		openai.WithAPIKey(apiKey),
		openai.WithExtraFields(map[string]any{
			"reasoning_split": true,
			"thinking":        map[string]string{"type": "disabled"},
		}),
	)

	agent := llmagent.New(
		"ui-designer",
		llmagent.WithModel(llm),
		llmagent.WithDescription("生成受控的 UI 组件树"),
		llmagent.WithInstruction(
			"你是 UI 设计师。根据用户需求生成一个简洁页面。"+
				"只能使用 hero、text、card、stat、button 组件；"+
				"颜色必须是 CSS 十六进制色；组件不超过 8 个。",
		),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			MaxTokens:   model.IntPtr(1200),
			Temperature: model.Float64Ptr(0.4),
			Stream:      false,
		}),
		llmagent.WithStructuredOutputJSON(
			new(UIPage),
			false,
			"一个由受控组件构成的 UI 页面",
		),
	)

	r := runner.NewRunner(
		"generative-ui-demo",
		agent,
		runner.WithSessionService(inmemory.NewSessionService()),
	)
	defer r.Close()

	s := &server{runner: r}
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/generate", s.generate)
	mux.Handle("/", http.FileServer(http.FS(assets)))

	log.Println("打开 http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	var input generateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 无效")
		return
	}
	if input.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt 不能为空")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	sessionID := fmt.Sprintf("ui-%d", time.Now().UnixNano())
	events, err := s.runner.Run(
		ctx,
		"browser-user",
		sessionID,
		model.NewUserMessage(input.Prompt),
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var page *UIPage
	for event := range events {
		if event.Error != nil {
			writeError(w, http.StatusBadGateway, event.Error.Message)
			return
		}
		if output, ok := event.StructuredOutput.(*UIPage); ok {
			page = output
		}
	}
	if page == nil {
		writeError(w, http.StatusBadGateway, errors.New("模型未返回结构化 UI").Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(page)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
