package gateway

import (
	"context"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Client wraps runner.Runner for non-streaming IM usage.
type Client struct {
	runner runner.Runner
}

func NewClient(r runner.Runner) *Client {
	return &Client{runner: r}
}

type SendTextInput struct {
	UserID    string
	SessionID string
	Text      string
}

// SendText sends a message and returns the assistant's final text.
// Non-streaming — waits for the full reply.
func (c *Client) SendText(ctx context.Context, in SendTextInput) (string, error) {
	eventCh, err := c.runner.Run(
		ctx,
		in.UserID,
		in.SessionID,
		model.NewUserMessage(in.Text),
		agent.WithStream(false),
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
