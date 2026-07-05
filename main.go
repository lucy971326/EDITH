package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github-agent/agent"
	"github-agent/github/auth"

	"github.com/google/go-github/v69/github"
)

func loadEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && os.Getenv(parts[0]) == "" {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func getPayload(r *http.Request, secret []byte) ([]byte, error) {
	if len(secret) == 0 {
		defer r.Body.Close()
		return io.ReadAll(r.Body)
	}
	return github.ValidatePayload(r, secret)
}

func main() {
	loadEnv(".env")

	auth.Init(os.Getenv("GITHUB_APP_ID"), "private_key.pem")
	webhookSecret := []byte(os.Getenv("GITHUB_WEBHOOK_SECRET"))

	http.HandleFunc("/webhook/github", func(w http.ResponseWriter, r *http.Request) {
		payload, err := getPayload(r, webhookSecret)
		if err != nil {
			log.Printf("签名验证失败: %v", err)
			http.Error(w, "invalid signature", 401)
			return
		}

		event, err := github.ParseWebHook(r.Header.Get("X-GitHub-Event"), payload)
		if err != nil {
			log.Printf("解析事件失败: %v", err)
			http.Error(w, "bad payload", 400)
			return
		}

		switch e := event.(type) {
		case *github.IssuesEvent:
			if e.GetAction() == "opened" {
				log.Printf("📌 Issue 创建: %s #%d", e.GetRepo().GetFullName(), e.GetIssue().GetNumber())
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetIssue().GetNumber(),
					Title:           e.GetIssue().GetTitle(),
					Body:            e.GetIssue().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
				})
			}

		case *github.IssueCommentEvent:
			if e.GetAction() == "created" {
				user := e.GetComment().GetUser().GetLogin()
				if strings.HasSuffix(user, "[bot]") {
					log.Printf("忽略 bot 评论: %s", user)
					break
				}
				log.Printf("💬 Issue 评论: %s #%d", e.GetRepo().GetFullName(), e.GetIssue().GetNumber())
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetIssue().GetNumber(),
					Title:           e.GetIssue().GetTitle(),
					Body:            e.GetIssue().GetBody(),
					Comment:         e.GetComment().GetBody(),
					User:            e.GetComment().GetUser().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
				})
			}

		case *github.PullRequestEvent:
			log.Printf("📋 PR %s: %s #%d", e.GetAction(), e.GetRepo().GetFullName(), e.GetPullRequest().GetNumber())
			if e.GetAction() == "opened" || e.GetAction() == "synchronize" {
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetPullRequest().GetNumber(),
					Title:           e.GetPullRequest().GetTitle(),
					Body:            e.GetPullRequest().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
				})
			}

		case *github.PullRequestReviewEvent:
			if e.GetAction() == "submitted" {
				log.Printf("📋 PR Review: %s #%d", e.GetRepo().GetFullName(), e.GetPullRequest().GetNumber())
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetPullRequest().GetNumber(),
					Title:           e.GetPullRequest().GetTitle(),
					Body:            e.GetPullRequest().GetBody(),
					Comment:         e.GetReview().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
				})
			}

		case *github.PullRequestReviewCommentEvent:
			if e.GetAction() == "created" {
				log.Printf("📋 PR 行级评论: %s #%d", e.GetRepo().GetFullName(), e.GetPullRequest().GetNumber())
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetPullRequest().GetNumber(),
					Title:           e.GetPullRequest().GetTitle(),
					Body:            e.GetPullRequest().GetBody(),
					Comment:         e.GetComment().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
				})
			}
		}

		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	log.Printf("🚀 启动在 :2026")
	log.Fatal(http.ListenAndServe(":2026", nil))
}
