package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// GitHubToolSet 提供 GitHub 操作的专用工具。
// 绕过 shell 引号问题，用 exec.Command 直接传参数。
type GitHubToolSet struct {
	name  string
	tools []tool.Tool
}

func NewGitHubToolSet() *GitHubToolSet {
	s := &GitHubToolSet{name: "github"}
	s.tools = []tool.Tool{
		s.makeCommentOnIssue(),
		s.makeCommentOnPR(),
		s.makeGetPRDiff(),
		s.makeCreatePR(),
	}
	return s
}

func (s *GitHubToolSet) Tools(_ context.Context) []tool.Tool { return s.tools }
func (s *GitHubToolSet) Close() error                        { return nil }
func (s *GitHubToolSet) Name() string                        { return s.name }

// ── ghBinary: 找到 gh 可执行文件 ──
func ghBinary() string {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("gh.exe"); err == nil {
			return p
		}
		return "gh.exe"
	}
	return "gh"
}

// ── comment_on_issue ──
// 输入用 --body-file 写入临时文件，绕过 shell 多行字符串引号地狱

type commentOnIssueArgs struct {
	Repo        string `json:"repo" jsonschema:"description=仓库全名，如 owner/repo"`
	IssueNumber int    `json:"issue_number" jsonschema:"description=Issue 编号"`
	Body        string `json:"body" jsonschema:"description=评论内容，支持 Markdown"`
}

type commentOnIssueResult struct {
	URL string `json:"url"`
}

func (s *GitHubToolSet) makeCommentOnIssue() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args commentOnIssueArgs) (commentOnIssueResult, error) {
			// 写入临时文件，避免 shell 引号问题
			tmp, err := os.CreateTemp("", "gh-comment-*.md")
			if err != nil {
				return commentOnIssueResult{}, fmt.Errorf("创建临时文件失败: %w", err)
			}
			defer os.Remove(tmp.Name())

			if _, err := tmp.WriteString(args.Body); err != nil {
				return commentOnIssueResult{}, fmt.Errorf("写入评论失败: %w", err)
			}
			tmp.Close()

			// 直接用 exec.Command 传参，不走 shell，参数安全
			issueStr := strconv.Itoa(args.IssueNumber)
			cmd := exec.CommandContext(ctx, ghBinary(),
				"issue", "comment", issueStr,
				"--repo", args.Repo,
				"--body-file", tmp.Name(),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return commentOnIssueResult{}, fmt.Errorf("gh 执行失败: %w\n%s", err, string(out))
			}

			return commentOnIssueResult{URL: trimNewline(string(out))}, nil
		},
		function.WithName("comment_on_issue"),
		function.WithDescription("在 Issue 上发布评论，内容支持 Markdown。发布后返回评论 URL。"),
	)
}

// ── comment_on_pr ──

type commentOnPRArgs struct {
	Repo     string `json:"repo" jsonschema:"description=仓库全名，如 owner/repo"`
	PRNumber int    `json:"pr_number" jsonschema:"description=PR 编号"`
	Body     string `json:"body" jsonschema:"description=评论内容，支持 Markdown"`
}

type commentOnPRResult struct {
	URL string `json:"url"`
}

func (s *GitHubToolSet) makeCommentOnPR() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args commentOnPRArgs) (commentOnPRResult, error) {
			tmp, err := os.CreateTemp("", "gh-pr-comment-*.md")
			if err != nil {
				return commentOnPRResult{}, fmt.Errorf("创建临时文件失败: %w", err)
			}
			defer os.Remove(tmp.Name())

			if _, err := tmp.WriteString(args.Body); err != nil {
				return commentOnPRResult{}, fmt.Errorf("写入评论失败: %w", err)
			}
			tmp.Close()

			cmd := exec.CommandContext(ctx, ghBinary(),
				"pr", "comment", strconv.Itoa(args.PRNumber),
				"--repo", args.Repo,
				"--body-file", tmp.Name(),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return commentOnPRResult{}, fmt.Errorf("gh 执行失败: %w\n%s", err, string(out))
			}

			return commentOnPRResult{URL: trimNewline(string(out))}, nil
		},
		function.WithName("comment_on_pr"),
		function.WithDescription("在 PR 上发布评论，内容支持 Markdown。发布后返回评论 URL。"),
	)
}

// ── get_pr_diff ──

type getPRDiffArgs struct {
	Repo     string `json:"repo" jsonschema:"description=仓库全名，如 owner/repo"`
	PRNumber int    `json:"pr_number" jsonschema:"description=PR 编号"`
}

type getPRDiffResult struct {
	Diff string `json:"diff"`
}

func (s *GitHubToolSet) makeGetPRDiff() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args getPRDiffArgs) (getPRDiffResult, error) {
			cmd := exec.CommandContext(ctx, ghBinary(),
				"pr", "diff", strconv.Itoa(args.PRNumber),
				"--repo", args.Repo,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return getPRDiffResult{}, fmt.Errorf("gh 执行失败: %w\n%s", err, string(out))
			}

			return getPRDiffResult{Diff: string(out)}, nil
		},
		function.WithName("get_pr_diff"),
		function.WithDescription("获取 PR 的代码 diff，用于审查代码变更。"),
	)
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// ── create_pr ──

type createPRArgs struct {
	Repo       string `json:"repo" jsonschema:"description=仓库全名，如 owner/repo"`
	Title      string `json:"title" jsonschema:"description=PR 标题"`
	Body       string `json:"body" jsonschema:"description=PR 描述，支持 Markdown，可用 Closes #N 关联 Issue"`
	HeadBranch string `json:"head_branch" jsonschema:"description=源分支名"`
	BaseBranch string `json:"base_branch" jsonschema:"description=目标分支名，默认 main"`
}

type createPRResult struct {
	URL  string `json:"url"`
	PRID int    `json:"pr_id"`
}

func (s *GitHubToolSet) makeCreatePR() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, args createPRArgs) (createPRResult, error) {
			tmp, err := os.CreateTemp("", "gh-pr-body-*.md")
			if err != nil {
				return createPRResult{}, fmt.Errorf("创建临时文件失败: %w", err)
			}
			defer os.Remove(tmp.Name())

			if _, err := tmp.WriteString(args.Body); err != nil {
				return createPRResult{}, fmt.Errorf("写入 PR 描述失败: %w", err)
			}
			tmp.Close()

			base := args.BaseBranch
			if base == "" {
				base = "main"
			}

			cmd := exec.CommandContext(ctx, ghBinary(),
				"pr", "create",
				"--repo", args.Repo,
				"--title", args.Title,
				"--body-file", tmp.Name(),
				"--head", args.HeadBranch,
				"--base", base,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return createPRResult{}, fmt.Errorf("gh 执行失败: %w\n%s", err, string(out))
			}

			url := trimNewline(string(out))
			return createPRResult{URL: url}, nil
		},
		function.WithName("create_pr"),
		function.WithDescription("创建 Pull Request。描述中写 Closes #N 可自动关联 Issue。返回 PR URL。"),
	)
}
