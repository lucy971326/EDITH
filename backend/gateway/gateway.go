package gateway

import (
	"context"
	"strings"

	"demo/sandbox"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Client wraps runner.Runner for non-streaming IM usage.
type Client struct {
	runner     runner.Runner
	basePrompt string
	provider   sandbox.BackendProvider
}

func NewClient(r runner.Runner, basePrompt string, provider sandbox.BackendProvider) *Client {
	return &Client{
		runner:     r,
		basePrompt: strings.TrimSpace(basePrompt),
		provider:   provider,
	}
}

type SendTextInput struct {
	UserID    string
	SessionID string
	Text      string
}

// SendText sends a message and returns the assistant's final text.
// Non-streaming — waits for the full reply.
func (c *Client) SendText(ctx context.Context, in SendTextInput) (string, error) {
	runOpts := []agent.RunOption{agent.WithStream(false)}
	if c.provider != nil {
		overview, err := c.provider.LoadUserSkillsOverview(ctx, in.UserID)
		if err != nil {
			return "", err
		}
		if overview != "" {
			runOpts = append(runOpts, agent.WithGlobalInstruction(
				c.basePrompt+"\n\n## 可用用户 Skills\n\n"+overview,
			))
		}
	}

	eventCh, err := c.runner.Run(
		ctx,
		in.UserID,
		in.SessionID,
		model.NewUserMessage(in.Text),
		runOpts...,
	)
	if err != nil {
		return "", err
	}

	var reply strings.Builder
	for ev := range eventCh {
		if ev.IsRunnerCompletion() {
			break
		}
		if ev.Response == nil || len(ev.Response.Choices) == 0 {
			continue
		}
		choice := ev.Response.Choices[0]
		if choice.Message.Role == model.RoleAssistant && choice.Message.Content != "" {
			reply.WriteString(choice.Message.Content)
		}
	}
	return reply.String(), nil
}
