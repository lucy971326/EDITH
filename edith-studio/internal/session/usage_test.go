package session

import (
	"context"
	"errors"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

func TestContextUsageReturnsLatestNonZeroPromptTokens(t *testing.T) {
	ctx := context.Background()
	service := inmemory.NewSessionService()
	module := &Module{service: service}
	key := frameworksession.Key{AppName: studioAppName, UserID: "workspace-one", SessionID: "session-one"}
	storedSession, err := service.CreateSession(ctx, key, frameworksession.StateMap{})
	if err != nil {
		t.Fatal(err)
	}
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "user", &model.Response{
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleUser, Content: "你好"}}},
	}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "edith", &model.Response{
		Usage:   &model.Usage{PromptTokens: 512, CompletionTokens: 64, TotalTokens: 576},
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "你好！"}}},
	}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-2", "edith", &model.Response{
		Usage:   &model.Usage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "继续"}}},
	}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-3", "edith", &model.Response{
		Usage:   &model.Usage{PromptTokens: 1280, CompletionTokens: 96, TotalTokens: 1376},
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "更长了"}}},
	}))

	usage, err := module.ContextUsage(ctx, "workspace-one", "session-one")
	if err != nil {
		t.Fatal(err)
	}
	if usage.PromptTokens != 1280 {
		t.Fatalf("PromptTokens = %d, want 1280", usage.PromptTokens)
	}
}

func TestContextUsageReturnsZeroWithoutOfficialUsage(t *testing.T) {
	ctx := context.Background()
	service := inmemory.NewSessionService()
	module := &Module{service: service}
	key := frameworksession.Key{AppName: studioAppName, UserID: "workspace-one", SessionID: "session-one"}
	storedSession, err := service.CreateSession(ctx, key, frameworksession.StateMap{})
	if err != nil {
		t.Fatal(err)
	}
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "user", &model.Response{
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleUser, Content: "你好"}}},
	}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "edith", &model.Response{
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "没有用量字段"}}},
	}))

	usage, err := module.ContextUsage(ctx, "workspace-one", "session-one")
	if err != nil {
		t.Fatal(err)
	}
	if usage.PromptTokens != 0 {
		t.Fatalf("PromptTokens = %d, want 0", usage.PromptTokens)
	}
}

func TestContextUsageIsolatedAcrossWorkspaces(t *testing.T) {
	ctx := context.Background()
	service := inmemory.NewSessionService()
	module := &Module{service: service}
	key := frameworksession.Key{AppName: studioAppName, UserID: "workspace-one", SessionID: "session-one"}
	storedSession, err := service.CreateSession(ctx, key, frameworksession.StateMap{})
	if err != nil {
		t.Fatal(err)
	}
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "edith", &model.Response{
		Usage:   &model.Usage{PromptTokens: 512, TotalTokens: 512},
		Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "hi"}}},
	}))

	_, err = module.ContextUsage(ctx, "workspace-two", "session-one")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("error = %v, want ErrSessionNotFound", err)
	}
}
