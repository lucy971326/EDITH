package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github-agent/agent"
	"github-agent/github/auth"

	"github.com/google/go-github/v69/github"
	"trpc.group/trpc-go/trpc-agent-go/server/agui"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// PR Review 延迟：等行级评论取消，避免重复触发
var pendingReview = make(map[string]*time.Timer)

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

	mux := http.NewServeMux()

	// ── Webhook（GitHub 事件入口） ──
	mux.HandleFunc("/webhook/github", func(w http.ResponseWriter, r *http.Request) {
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
					EventType:       "issue",
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
					EventType:       "issue_comment",
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
					HeadBranch:      e.GetPullRequest().GetHead().GetRef(),
					BaseBranch:      e.GetPullRequest().GetBase().GetRef(),
					EventType:       "pr",
				})
			}

		case *github.PullRequestReviewEvent:
			if e.GetAction() == "submitted" {
				sessionID := fmt.Sprintf("%s#%d", e.GetRepo().GetFullName(), e.GetPullRequest().GetNumber())
				log.Printf("📋 PR Review: %s (等待 3s 看有没有行级评论)", sessionID)
				pending := agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetPullRequest().GetNumber(),
					Title:           e.GetPullRequest().GetTitle(),
					Body:            e.GetPullRequest().GetBody(),
					Comment:         e.GetReview().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
					HeadBranch:      e.GetPullRequest().GetHead().GetRef(),
					BaseBranch:      e.GetPullRequest().GetBase().GetRef(),
					EventType:       "pr_review",
				}
				pendingReview[sessionID] = time.AfterFunc(3*time.Second, func() {
					log.Printf("📋 3s 到期，没有行级评论，处理 PR Review")
					agent.Analyze(pending)
				})
			}

		case *github.PullRequestReviewCommentEvent:
			if e.GetAction() == "created" {
				sessionID := fmt.Sprintf("%s#%d", e.GetRepo().GetFullName(), e.GetPullRequest().GetNumber())
				if t, ok := pendingReview[sessionID]; ok {
					t.Stop()
					delete(pendingReview, sessionID)
					log.Printf("📋 取消 PR Review，改控行级评论: %s", sessionID)
				}
				agent.Analyze(agent.EventContext{
					Repo:            e.GetRepo().GetFullName(),
					Number:          e.GetPullRequest().GetNumber(),
					Title:           e.GetPullRequest().GetTitle(),
					Body:            e.GetPullRequest().GetBody(),
					Comment:         e.GetComment().GetBody(),
					User:            e.GetSender().GetLogin(),
					InstallationID:  e.GetInstallation().GetID(),
					HeadBranch:      e.GetPullRequest().GetHead().GetRef(),
					BaseBranch:      e.GetPullRequest().GetBase().GetRef(),
					EventType:       "pr_comment",
					File:            e.GetComment().GetPath(),
					Line:            e.GetComment().GetLine(),
					DiffHunk:        e.GetComment().GetDiffHunk(),
				})
			}
		}

		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	// ── AG-UI（网页对话，共享 :2026 端口） ──
	// 更具体的路由优先匹配：/webhook/github 和 /api/sessions 先抢到，其他走到 AG-UI
	aguiServer, err := agui.New(
		agent.GetRunner(),
		agui.WithPath("/chat"),
		agui.WithSessionService(agent.SessionService),
		agui.WithAppName("github-edith"),
		agui.WithMessagesSnapshotEnabled(true),
		agui.WithCancelEnabled(true),
		agui.WithAGUIRunnerOptions(
			aguirunner.WithUserIDResolver(func(_ context.Context, _ *adapter.RunAgentInput) (string, error) {
				return "default", nil
			}),
		),
	)
	if err != nil {
		log.Fatalf("AG-UI 启动失败: %v", err)
	}
	mux.Handle("/", aguiServer.Handler())

	// ── 会话列表 API ──
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		sessions, err := agent.SessionService.ListSessions(r.Context(), session.UserKey{
			AppName: "github-edith",
			UserID:  "default",
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	})

	// ── 会话历史 API（直接从 Events 读，绕过 tracker） ──
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "missing session_id", 400)
			return
		}
		// 先查 github-edith，再查 github-agent（兼容旧数据）
		var sess *session.Session
		for _, app := range []string{"github-edith", "github-agent"} {
			s, _ := agent.SessionService.GetSession(r.Context(), session.Key{
				AppName:   app,
				UserID:    "default",
				SessionID: sessionID,
			})
			if s != nil {
				sess = s
				break
			}
		}
		if sess == nil {
			http.Error(w, "session not found", 404)
			return
		}
		type msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		var messages []msg
		for _, e := range sess.Events {
			if e.Response == nil || len(e.Response.Choices) == 0 {
				continue
			}
			c := e.Response.Choices[0]
			role := string(c.Message.Role)
			if role != "user" && role != "assistant" {
				continue
			}
			content := c.Message.Content
			if content == "" {
				continue
			}
			messages = append(messages, msg{Role: role, Content: content})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	log.Printf("🚀 EDITH 启动  :2026\n"+
		"  GitHub Webhook → /webhook/github\n"+
		"  AG-UI 对话    → POST /\n"+
		"  会话 API      → GET /api/sessions")
	log.Fatal(http.ListenAndServe(":2026", mux))
}
