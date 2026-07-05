package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github-agent/agent/backend"
	"github-agent/agent/tools"
	"github-agent/github/auth"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// EventContext 描述 GitHub 上发生了什么事件
type EventContext struct {
	Repo           string // 仓库全名，如 lucy971326/sysdash-tui
	Number         int    // Issue/PR 编号
	Title          string // Issue/PR 标题
	Body           string // Issue/PR 描述
	Comment        string // 评论内容（如有）
	User           string // 触发者
	InstallationID int64  // App 安装 ID，操作 API 时需要
	HeadBranch     string // PR head 分支（仅 PR 事件）
	BaseBranch     string // PR base 分支（仅 PR 事件）
	EventType      string // 事件类型：issue / issue_comment / pr / pr_review / pr_comment
	File           string // 行级评论所在文件（仅 PR review comment）
	Line           int    // 行级评论所在行号（仅 PR review comment）
	DiffHunk       string // 行级评论的 diff 上下文（仅 PR review comment）
}

var (
	r       runner.Runner
	once    sync.Once
	lastRun = make(map[string]time.Time) // sessionID → 上次触发时间
)

func workspaceDir() string {
	w := os.Getenv("WORKSPACE_DIR")
	if w == "" {
		w = "./workspace"
	}
	os.MkdirAll(w, 0755)
	return w
}

func ensureRunner() {
	once.Do(func() {
		m := openai.New(os.Getenv("LLM_MODEL"),
			openai.WithBaseURL(os.Getenv("LLM_BASE_URL")),
			openai.WithAPIKey(os.Getenv("LLM_API_KEY")),
		)

		dir := workspaceDir()
		b := backend.NewLocalBackend(dir)
		ft := tools.NewFileToolSet(b)
		gh := tools.NewGitHubToolSet()

		ag := llmagent.New(
			"github-edith",
			llmagent.WithModel(m),
			llmagent.WithInstruction(
				"你是 @EDITH，一个 GitHub 代码助手。\n"+
					"第一要务：如果 Issue/评论中没有 @EDITH 字样，说明不是喊你的，直接忽略不要处理。\n"+
					"当前 workspace 目录："+dir+"\n"+
					"所有文件路径均以 workspace 为根，使用相对路径。执行命令时 work_dir 默认传 \".\" 即可。\n"+
					"clone 仓库时目标目录只写仓库名，不要写 workspace 前缀（例如 gh repo clone owner/repo myrepo，不要写 workspace/myrepo）。\n"+
					"\n"+
					"回复 Issue 评论时使用 comment_on_issue，回复 PR 评论时使用 comment_on_pr。\n"+
					"创建 PR 时使用 create_pr（描述写 Closes #N 自动关联 Issue）。\n"+
					"\n"+
					"## Bug Fix 流程（Issue 要求修 bug 时）\n"+
					"1. gh repo clone 到 workspace\n"+
					"2. 读代码、分析问题\n"+
					"3. git checkout -b fix/xxx 创建分支\n"+
					"4. write_file 修改代码\n"+
					"5. exec_command: git add -A && git commit -m \"fix: ...\"\n"+
					"6. exec_command: git push origin fix/xxx\n"+
					"7. create_pr 创建 PR，描述中写 Closes #N 关联原 Issue\n"+
					"8. comment_on_issue 通知用户 PR 已创建\n"+
					"\n"+
					"审 PR 时：先 clone 仓库，再用 get_pr_diff 拿 diff，结合代码全貌分析，最后用 comment_on_pr 回复。\n"+
					"仔细阅读 Issue/PR，需要时读代码、搜代码、执行命令来辅助分析。",
			),
			llmagent.WithToolSets([]tool.ToolSet{ft, gh}),
		)

		sessionService := inmemory.NewSessionService()
		r = runner.NewRunner("github-agent", ag, runner.WithSessionService(sessionService))
	})
}

func Analyze(info EventContext) {
	ensureRunner()

	// 防抖：同一 session + 同一事件类型 5秒内不重复触发
	sessionID := fmt.Sprintf("%s#%d", info.Repo, info.Number)
	debounceKey := sessionID + ":" + info.EventType
	if time.Since(lastRun[debounceKey]) < 5*time.Second {
		log.Printf("防抖: %s 在 5s 内已触发，跳过", debounceKey)
		return
	}
	lastRun[debounceKey] = time.Now()

	// 设置 GITHUB_TOKEN，gh / git 可直接操作仓库
	if token, err := auth.GetInstallationToken(context.Background(), info.InstallationID); err == nil {
		os.Setenv("GITHUB_TOKEN", token)
	}

	// 根据事件类型构造 prompt
	isPR := info.HeadBranch != ""
	eventType := "Issue"
	if isPR {
		eventType = "PR"
	}
	prompt := fmt.Sprintf(
		"[%s #%d] %s\n仓库: %s\n描述: %s\n评论: %s\n评论者: %s\n",
		eventType, info.Number, info.Title, info.Repo, info.Body, info.Comment, info.User,
	)
	if isPR {
		prompt += fmt.Sprintf("分支: %s → %s\n", info.HeadBranch, info.BaseBranch)
	}
	if info.File != "" {
		prompt += fmt.Sprintf("文件: %s  行号: %d\nDiff 上下文:\n%s\n", info.File, info.Line, info.DiffHunk)
	}
	prompt += "\n分析:"

	eventChan, err := r.Run(
		context.Background(),
		info.User,
		fmt.Sprintf("%s#%d", info.Repo, info.Number),
		model.NewUserMessage(prompt),
	)
	if err != nil {
		fmt.Printf("Agent 运行失败: %v\n", err)
		return
	}

	fmt.Println("\n--- Agent 分析 ---")
	for event := range eventChan {
		if event.IsRunnerCompletion() {
			break
		}
		if event.Error != nil {
			fmt.Printf("\n⚠️  %s", event.Error.Message)
			continue
		}
		if len(event.Response.Choices) == 0 {
			continue
		}
		c := event.Response.Choices[0]

		if text := c.Delta.ReasoningContent; text != "" {
			fmt.Printf("\n💭 %s", text)
		}
		if text := c.Message.ReasoningContent; text != "" {
			fmt.Printf("\n💭 %s", text)
		}
		if len(c.Message.ToolCalls) > 0 {
			for _, tc := range c.Message.ToolCalls {
				fmt.Printf("\n🔧 %s(%s)", tc.Function.Name, string(tc.Function.Arguments))
			}
		}
		if text := c.Delta.Content; text != "" {
			fmt.Printf("\n🤖 %s", text)
		}
		if text := c.Message.Content; text != "" {
			fmt.Printf("\n🤖 %s", text)
		}
	}
	fmt.Println("\n--- 分析完成 ---")
}
