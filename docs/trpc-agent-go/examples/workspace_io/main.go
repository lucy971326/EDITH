//
// Tencent is pleased to support the open source community by making
// trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//
//

// Package main 演示如何在 LLMAgent 调用后，将 workspace 文件镜像到
// 调用方管理的存储中，仅使用 codeexecutor/workspaceio.Workspace
// 和普通的 AgentCallbacks。
//
// LLMAgent 没有提供专门的调用后刷写选项——
// 框架将时机、错误类型和预算选择留给调用方。
// 这里展示的模式很简单：
//
//  1. 从 ctx 中解析 Workspace。
//  2. 调用 ws.Collect 枚举匹配的文件。
//  3. 循环并将每个 *workspaceio.File 传递给你的 sink。
//
// LocalCodeExecutor 使示例完全自包含（无 Docker、无远程沙箱）；
// "用户级 skill 存储"是 ./skills_store 下的主机目录。
// 实际部署中会将 `directorySink` 替换为
// 数据库 / 对象存储 / HTTP 服务。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/codeexecutor"
	localexec "trpc.group/trpc-go/trpc-agent-go/codeexecutor/local"
	"trpc.group/trpc-go/trpc-agent-go/codeexecutor/workspaceio"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

var (
	modelName = flag.String("model", "deepseek-v4-flash", "使用的模型名称")
	storeDir  = flag.String("store", "./skills_store",
		"用作用户级 skill 存储的主机目录")
	prompt = flag.String(
		"prompt",
		"Say a short hello so I can verify the agent finished.",
		"发送给 agent 的用户提示词",
	)
)

func main() {
	flag.Parse()

	absStore, err := filepath.Abs(*storeDir)
	if err != nil {
		log.Fatalf("resolve store dir: %v", err)
	}
	if err := os.MkdirAll(absStore, 0o755); err != nil {
		log.Fatalf("create store dir: %v", err)
	}

	fmt.Printf("Workspace I/O demo\n")
	fmt.Printf("- model:        %s\n", *modelName)
	fmt.Printf("- skill store:  %s\n", absStore)
	fmt.Println(strings.Repeat("=", 60))

	sink := newDirectorySink(absStore)

	cb := agent.NewCallbacks()
	cb.RegisterBeforeAgent(seedWorkspaceProfile)
	cb.RegisterAfterAgent(mirrorSkillsAfterAgent(sink))

	a := llmagent.New(
		"workspace-flush-demo",
		llmagent.WithModel(openai.New(*modelName)),
		llmagent.WithDescription(
			"Demonstrates programmatic workspaceio.Workspace usage.",
		),
		llmagent.WithInstruction(
			"You are a helpful assistant. Keep replies short.",
		),
		// LocalCodeExecutor 为每次调用提供独立的 work/ 根目录。
		// 任何后端（container、pcg123 NFS、cube）通过
		// codeexecutor.Engine 的工作方式都相同。
		llmagent.WithCodeExecutor(localexec.New()),
		llmagent.WithAgentCallbacks(cb),
	)

	r := runner.NewRunner("workspace-flush-demo-app", a)
	defer r.Close()

	ctx := context.Background()
	events, err := r.Run(
		ctx, "demo-user", "demo-session",
		model.NewUserMessage(*prompt),
	)
	if err != nil {
		log.Fatalf("run agent: %v", err)
	}
	drainEvents(events)

	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("Skill store after invocation:")
	listStore(absStore)
}

// seedWorkspaceProfile 预先在 workspace 中填充两个 SKILL.md 文件，
// 模拟一个 agent 从某处加载用户配置并在模型运行前将其投射到 workspace 中。
func seedWorkspaceProfile(
	ctx context.Context,
	args *agent.BeforeAgentArgs,
) (*agent.BeforeAgentResult, error) {
	ws, ok := workspaceio.WorkspaceFromContext(ctx)
	if !ok {
		log.Printf(
			"Workspace 不可用；请检查是否配置了 " +
				"WithCodeExecutor",
		)
		return nil, nil
	}
	skills := []codeexecutor.PutFile{
		{
			Path:    "skills/echoer/SKILL.md",
			Content: []byte("# Echoer\n\nReplies with the same text.\n"),
		},
		{
			Path:    "skills/greeter/SKILL.md",
			Content: []byte("# Greeter\n\nGreets the user politely.\n"),
		},
	}
	if err := ws.PutFiles(ctx, skills...); err != nil {
		return nil, fmt.Errorf("seed workspace skills: %w", err)
	}
	for _, f := range skills {
		log.Printf("seeded workspace file: %s (%d bytes)", f.Path, len(f.Content))
	}
	return nil, nil
}

// mirrorSkillsAfterAgent 返回一个 AfterAgent 回调，将 workspace 中
// 所有 skills/*/SKILL.md 复制到 sink 中。整个模式就是一次 Collect
// 加上一个 sink 循环——故意没有使用框架辅助函数。
func mirrorSkillsAfterAgent(sink *directorySink) agent.AfterAgentCallbackStructured {
	return func(
		ctx context.Context, args *agent.AfterAgentArgs,
	) (*agent.AfterAgentResult, error) {
		// 当 agent 本身失败时跳过镜像；此时 workspace
		// 状态不可靠。
		if args.Error != nil {
			return nil, nil
		}
		ws, ok := workspaceio.WorkspaceFromContext(ctx)
		if !ok {
			return nil, nil
		}
		files, err := ws.Collect(ctx, "skills/*/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("collect skills: %w", err)
		}
		for _, f := range files {
			if f.Truncated {
				return nil, fmt.Errorf(
					"%s was truncated by the executor (size=%d)",
					f.Path, f.SizeBytes,
				)
			}
			if err := validateSkillMarkdown(f); err != nil {
				return nil, err
			}
			if err := sink.Save(ctx, args.Invocation, f); err != nil {
				return nil, fmt.Errorf("sink %s: %w", f.Path, err)
			}
		}
		return nil, nil
	}
}

// validateSkillMarkdown 拒绝空的或没有标题的 SKILL.md 文件。
// 实际的验证器会解析 YAML frontmatter、检查必需的标题等。
func validateSkillMarkdown(file *workspaceio.File) error {
	if len(file.Data) == 0 {
		return fmt.Errorf("%s is empty", file.Path)
	}
	if !strings.Contains(string(file.Data), "#") {
		return fmt.Errorf("%s has no markdown heading", file.Path)
	}
	return nil
}

// directorySink 将每个镜像的 workspace 文件持久化到 root/<userID>/<path> 下，
// 保留 workspace 相对目录结构。
type directorySink struct {
	root string
}

func newDirectorySink(root string) *directorySink { return &directorySink{root: root} }

func (s *directorySink) Save(
	_ context.Context,
	inv *agent.Invocation,
	file *workspaceio.File,
) error {
	userID := "anonymous"
	if inv != nil && inv.Session != nil {
		userID = inv.Session.UserID
	}
	// 拒绝写入 s.root 之外的路径。file.Path 来自 workspace
	// 收集器，应该是 workspace 相对路径，但本示例是复制粘贴素材——
	// 保留显式的包含检查，确保用户代码默认安全。
	// filepath.IsLocal (Go 1.20+) 一次性拒绝绝对路径、".." 和
	// Windows UNC/volume 转义。
	rel := filepath.Join(userID, file.Path)
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("directorySink: refusing to write outside sink root: %q", rel)
	}
	dst := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, file.Data, 0o644); err != nil {
		return err
	}
	log.Printf("mirrored %s -> %s (%d bytes)", file.Path, dst, len(file.Data))
	return nil
}

func drainEvents(events <-chan *event.Event) {
	for ev := range events {
		if ev.Error != nil {
			log.Printf("agent error: %s", ev.Error.Message)
		}
		if len(ev.Response.Choices) > 0 {
			c := ev.Response.Choices[0]
			if c.Message.Content != "" {
				fmt.Printf("[assistant] %s\n", c.Message.Content)
			}
		}
		if ev.Done {
			return
		}
	}
}

func listStore(root string) {
	err := filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		fmt.Printf("- %s (%d bytes)\n", rel, info.Size())
		return nil
	})
	if err != nil {
		log.Printf("walk store: %v", err)
	}
}
