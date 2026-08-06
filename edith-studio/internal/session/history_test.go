package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

func TestModuleGetRestoresChatHistory(t *testing.T) {
	ctx := context.Background()
	service := inmemory.NewSessionService()
	module := &Module{service: service}
	key := frameworksession.Key{AppName: studioAppName, UserID: "workspace-one", SessionID: "session-one"}
	storedSession, err := service.CreateSession(ctx, key, frameworksession.StateMap{})
	if err != nil {
		t.Fatal(err)
	}

	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "user", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleUser, Content: "请读取当前目录的所有文件，然后告诉我结构"}}}}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "edith", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, ReasoningContent: "我先查看目录", ToolCalls: []model.ToolCall{{ID: "tool-1", Function: model.FunctionDefinitionParam{Name: "Glob", Arguments: []byte(`{"pattern":"**/*"}`)}}}}}}}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "tool", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleTool, ToolID: "tool-1", ToolName: "Glob", Content: "main.go"}}}}))
	appendEvent(t, service, storedSession, event.NewResponseEvent("run-1", "edith", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, Content: "项目入口是 main.go。"}}}}))

	history, err := module.Get(ctx, "workspace-one", "session-one")
	if err != nil {
		t.Fatal(err)
	}
	if history.Session.Title != "请读取当前目录的所有文件，然后告诉我结构" {
		t.Fatalf("title = %q", history.Session.Title)
	}
	if _, err := time.Parse(time.RFC3339, history.Session.UpdatedAt); err != nil {
		t.Fatalf("updatedAt = %q: %v", history.Session.UpdatedAt, err)
	}
	if len(history.Messages) != 2 || history.Messages[0].Role != "user" || history.Messages[1].Role != "assistant" {
		t.Fatalf("messages = %#v", history.Messages)
	}
	blocks := history.Messages[1].Blocks
	if len(blocks) != 3 || blocks[0].Type != "reasoning" || blocks[1].Type != "tool" || blocks[1].Result != "main.go" || blocks[2].Content != "项目入口是 main.go。" {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestModuleFiltersWorkspaceAndReportsMissingSession(t *testing.T) {
	ctx := context.Background()
	service := inmemory.NewSessionService()
	module := &Module{service: service}
	for _, userID := range []string{"workspace-one", "workspace-two"} {
		key := frameworksession.Key{AppName: studioAppName, UserID: userID, SessionID: userID + "-session"}
		storedSession, err := service.CreateSession(ctx, key, frameworksession.StateMap{})
		if err != nil {
			t.Fatal(err)
		}
		appendEvent(t, service, storedSession, event.NewResponseEvent("run", "user", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleUser, Content: userID}}}}))
	}

	sessions, err := module.List(ctx, "workspace-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "workspace-one-session" {
		t.Fatalf("sessions = %#v", sessions)
	}
	_, err = module.Get(ctx, "workspace-one", "workspace-two-session")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Get error = %v", err)
	}
}

func appendEvent(t *testing.T, service *inmemory.SessionService, storedSession *frameworksession.Session, frameworkEvent *event.Event) {
	t.Helper()
	if err := service.AppendEvent(context.Background(), storedSession, frameworkEvent); err != nil {
		t.Fatal(err)
	}
}
