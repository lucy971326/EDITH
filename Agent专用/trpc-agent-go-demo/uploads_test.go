package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"demo/sandbox"
)

func TestUploadHandlerWritesToSessionWorkspace(t *testing.T) {
	provider, err := sandbox.NewLocalProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("user_id", "user-1")
	_ = writer.WriteField("session_id", "session-1")
	part, err := writer.CreateFormFile("file", "report.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	_, _ = part.Write([]byte("EDITH upload test"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	uploadHandler(provider).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var uploaded UploadedFile
	if err := json.NewDecoder(recorder.Body).Decode(&uploaded); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	backend, err := provider.GetBackend(context.Background(), sandbox.WorkspaceID{
		UserID: "user-1", SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("GetBackend() error = %v", err)
	}
	data, err := backend.ReadFile(context.Background(), uploaded.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", uploaded.Path, err)
	}
	if string(data) != "EDITH upload test" {
		t.Fatalf("stored content = %q", data)
	}
}
