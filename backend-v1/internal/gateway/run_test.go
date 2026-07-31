package gateway

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"edith/backend-v1/internal/models"
	"edith/backend-v1/internal/userconfig"
)

func TestGatewayResolvesWebMessage(t *testing.T) {
	gateway := &Gateway{users: newUserStore(t)}
	request, apiError := gateway.resolveMessage(IncomingMessage{
		Channel:        WebChannel,
		ExternalUserID: "clerk_123",
		SessionKey:     "browser-session",
		RequestID:      "request-1",
		Message:        "你好",
		ModelID:        "deepseek-v3",
	})
	if apiError != nil {
		t.Fatal(apiError)
	}
	if request.UserID != "clerk_123" || request.SessionID != "browser-session" || request.ModelID != "deepseek-v3" {
		t.Fatalf("request = %#v", request)
	}
}

func TestGatewayResolvesBoundChannelMessage(t *testing.T) {
	store := newUserStore(t)
	if err := store.BindChannelUser(context.Background(), userconfig.ChannelBinding{Channel: "feishu", ExternalUserID: "ou_123", UserID: "clerk_123"}); err != nil {
		t.Fatal(err)
	}
	gateway := &Gateway{users: store}
	request, apiError := gateway.resolveMessage(IncomingMessage{Channel: "feishu", ExternalUserID: "ou_123", RequestID: "request-1", Message: "你好"})
	if apiError != nil {
		t.Fatal(apiError)
	}
	if request.UserID != "clerk_123" || request.SessionID != "feishu:clerk_123" || request.ModelID != models.DefaultModelID {
		t.Fatalf("request = %#v", request)
	}
}

func TestGatewayRejectsUnboundChannelMessage(t *testing.T) {
	gateway := &Gateway{users: newUserStore(t)}
	_, apiError := gateway.resolveMessage(IncomingMessage{Channel: "feishu", ExternalUserID: "ou_123"})
	if apiError == nil || apiError.Type != "identity_not_bound" {
		t.Fatalf("error = %#v", apiError)
	}
}

func newUserStore(t *testing.T) *userconfig.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "edith.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := userconfig.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
func TestGatewayResolvesCronChannelMessage(t *testing.T) {
	store := newUserStore(t)
	gateway := &Gateway{users: store}
	request, apiError := gateway.resolveMessage(IncomingMessage{
		Channel:        "cron",
		ExternalUserID: "clerk_123",
		SessionKey:     "cron:job-1",
		RequestID:      "request-1",
		Message:        "生成日报",
	})
	if apiError != nil {
		t.Fatal(apiError)
	}
	// cron 是信任渠道：ExternalUserID 直接就是 Clerk 用户 ID，不查绑定表。
	if request.UserID != "clerk_123" || request.SessionID != "cron:job-1" || request.ModelID != models.DefaultModelID {
		t.Fatalf("request = %#v", request)
	}
}

func TestGatewayRejectsUnknownChannelWithoutBinding(t *testing.T) {
	gateway := &Gateway{users: newUserStore(t)}
	_, apiError := gateway.resolveMessage(IncomingMessage{Channel: "telegram", ExternalUserID: "tg_123"})
	if apiError == nil || apiError.Type != "identity_not_bound" {
		t.Fatalf("error = %#v", apiError)
	}
}
