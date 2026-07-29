package images

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

func TestMessageImagesUseSessionBoundRecordsAndDurableReferences(t *testing.T) {
	service := openTestService(t)
	ctx := context.Background()
	image, _, err := service.CreateUpload(ctx, "alice", UploadInput{
		SessionID: "session-1",
		MimeType:  "image/png",
		SizeBytes: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteUpload(ctx, "alice", image.ID); err != nil {
		t.Fatal(err)
	}

	message := model.NewUserMessage("看看这张图")
	messageContext, err := service.AddMessageImages(ctx, "alice", "session-1", []string{image.ID}, &message)
	if err != nil {
		t.Fatal(err)
	}
	if len(message.ContentParts) != 1 || message.ContentParts[0].Image == nil {
		t.Fatalf("message content parts = %#v", message.ContentParts)
	}

	persisted, changed := dehydrateMessage(messageContext, message)
	if !changed || persisted.ContentParts[0].Image.URL != Reference(image.ID) {
		t.Fatalf("persisted image URL = %q", persisted.ContentParts[0].Image.URL)
	}

	wrapped := &sessionService{images: service}
	hydrated, err := wrapped.hydrateMessage(ctx, persisted, "alice", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if hydrated.ContentParts[0].Image.URL != "https://cos.test/read/"+image.ID {
		t.Fatalf("hydrated image URL = %q", hydrated.ContentParts[0].Image.URL)
	}
}

func TestMessageImagesRejectOtherSession(t *testing.T) {
	service := openTestService(t)
	ctx := context.Background()
	image, _, err := service.CreateUpload(ctx, "alice", UploadInput{
		SessionID: "session-1",
		MimeType:  "image/png",
		SizeBytes: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteUpload(ctx, "alice", image.ID); err != nil {
		t.Fatal(err)
	}

	message := model.NewUserMessage("x")
	_, err = service.AddMessageImages(ctx, "alice", "session-2", []string{image.ID}, &message)
	if err == nil {
		t.Fatal("AddMessageImages succeeded for another session")
	}
}

func TestCompleteUploadDiscardsInvalidObject(t *testing.T) {
	storage := &invalidUploadCOS{}
	service := openTestServiceWithCOS(t, storage)
	ctx := context.Background()
	image, _, err := service.CreateUpload(ctx, "alice", UploadInput{
		SessionID: "session-1",
		MimeType:  "image/png",
		SizeBytes: 12,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.CompleteUpload(ctx, "alice", image.ID); err == nil {
		t.Fatal("CompleteUpload succeeded for an invalid object")
	}
	if !storage.deleted {
		t.Fatal("invalid COS object was not deleted")
	}
	if _, err := service.loadForUser(ctx, "alice", image.ID); err == nil {
		t.Fatal("invalid image reservation was not deleted")
	}
}

func TestSessionAppendKeepsHTTPSInMemoryAndMarkerInStore(t *testing.T) {
	ctx := context.Background()
	images := openTestService(t)
	image, _, err := images.CreateUpload(ctx, "alice", UploadInput{
		SessionID: "session-1",
		MimeType:  "image/png",
		SizeBytes: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := images.CompleteUpload(ctx, "alice", image.ID); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sessionsqlite.NewService(db)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })

	key := frameworksession.Key{AppName: "EDITH", UserID: "alice", SessionID: "session-1"}
	session, err := raw.CreateSession(ctx, key, nil)
	if err != nil {
		t.Fatal(err)
	}
	message := model.NewUserMessage("look")
	messageContext, err := images.AddMessageImages(ctx, "alice", "session-1", []string{image.ID}, &message)
	if err != nil {
		t.Fatal(err)
	}
	entry := event.NewResponseEvent("run", "chat", &model.Response{
		Choices: []model.Choice{{Message: message}},
	})

	wrapped := WrapSessionService(raw, images)
	if err := wrapped.AppendEvent(messageContext, session, entry); err != nil {
		t.Fatal(err)
	}
	inMemoryURL := session.Events[0].Response.Choices[0].Message.ContentParts[0].Image.URL
	if !strings.HasPrefix(inMemoryURL, "https://") {
		t.Fatalf("in-memory image URL = %q, want https URL", inMemoryURL)
	}

	persisted, err := raw.GetSession(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	persistedURL := persisted.Events[0].Response.Choices[0].Message.ContentParts[0].Image.URL
	if persistedURL != Reference(image.ID) {
		t.Fatalf("stored image URL = %q, want %q", persistedURL, Reference(image.ID))
	}

	history, err := wrapped.GetSession(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if historyURL := history.Events[0].Response.Choices[0].Message.ContentParts[0].Image.URL; historyURL != Reference(image.ID) {
		t.Fatalf("history image URL = %q, want durable reference", historyURL)
	}

	hydrated, err := wrapped.GetSession(WithHydratedSession(ctx), key)
	if err != nil {
		t.Fatal(err)
	}
	if hydratedURL := hydrated.Events[0].Response.Choices[0].Message.ContentParts[0].Image.URL; hydratedURL != "https://cos.test/read/"+image.ID {
		t.Fatalf("hydrated image URL = %q", hydratedURL)
	}
}

func openTestService(t *testing.T) *Service {
	return openTestServiceWithCOS(t, fakeCOS{})
}

func openTestServiceWithCOS(t *testing.T, storage cosStore) *Service {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "images.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{db: db, cos: storage}
	if err := service.createTable(context.Background()); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return service
}

type fakeCOS struct{}

func (fakeCOS) signPut(_ context.Context, objectKey string) (string, error) {
	return "https://cos.test/upload/" + objectKey, nil
}

func (fakeCOS) signGet(_ context.Context, objectKey string) (string, error) {
	return "https://cos.test/read/" + objectKey[len("chat-images/"):], nil
}

func (fakeCOS) head(_ context.Context, _ string) (objectInfo, error) {
	return objectInfo{MimeType: "image/png", SizeBytes: 12}, nil
}

func (fakeCOS) delete(_ context.Context, _ string) error { return nil }

type invalidUploadCOS struct {
	deleted bool
}

func (c *invalidUploadCOS) signPut(_ context.Context, objectKey string) (string, error) {
	return "https://cos.test/upload/" + objectKey, nil
}

func (c *invalidUploadCOS) signGet(_ context.Context, objectKey string) (string, error) {
	return "https://cos.test/read/" + objectKey, nil
}

func (*invalidUploadCOS) head(_ context.Context, _ string) (objectInfo, error) {
	return objectInfo{MimeType: "image/png", SizeBytes: 13}, nil
}

func (c *invalidUploadCOS) delete(_ context.Context, _ string) error {
	c.deleted = true
	return nil
}
