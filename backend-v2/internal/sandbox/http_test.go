package sandbox

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"edith/backend-v2/internal/httpx"
	_ "github.com/mattn/go-sqlite3"
)

func TestHTTPRejectsInvalidIdentityAndPathBeforeConnecting(t *testing.T) {
	db := newSandboxTestDB(t)
	h := &HTTP{workspaces: &service{db: db}}

	for _, target := range []string{
		"/internal/sandbox/files?sessionId=session-1",
		"/internal/sandbox/files?userId=user-1&sessionId=session-1&path=../secret",
		"/internal/sandbox/files?userId=user-1&sessionId=session-1&path=..%5Csecret",
		"/internal/sandbox/files/content?userId=user-1&sessionId=session-1&path=.",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, target, nil)
		if target == "/internal/sandbox/files/content?userId=user-1&sessionId=session-1&path=." {
			h.readContent(recorder, request)
		} else {
			h.listFiles(recorder, request)
		}
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestHTTPDoesNotCreateSandboxWhenMappingIsMissing(t *testing.T) {
	db := newSandboxTestDB(t)
	h := &HTTP{workspaces: &service{db: db}}
	recorder := httptest.NewRecorder()
	h.listFiles(recorder, httptest.NewRequest(http.MethodGet, "/internal/sandbox/files?userId=user-1&sessionId=session-1", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var response httpx.Error
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Type != "sandbox_not_found" {
		t.Fatalf("error type = %q, want sandbox_not_found", response.Type)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_sandboxes`).Scan(&count); err != nil {
		t.Fatalf("count mappings: %v", err)
	}
	if count != 0 {
		t.Fatalf("mapping count = %d, want 0", count)
	}
}

func TestIsPreviewableText(t *testing.T) {
	if !isPreviewableText([]byte("你好，EDITH")) {
		t.Fatal("valid UTF-8 text should be previewable")
	}
	for _, data := range [][]byte{{0xFF}, {'a', 0, 'b'}} {
		if isPreviewableText(data) {
			t.Fatalf("%v should not be previewable", data)
		}
	}
}

func TestHTTPUploadRejectsEmptyFileBeforeCreatingSandbox(t *testing.T) {
	db := newSandboxTestDB(t)
	h := &HTTP{workspaces: &service{db: db}}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "empty.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write(nil); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/sandbox/files/upload?userId=user-1&sessionId=session-1", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	h.uploadFile(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHTTPDownloadRejectsNonArtifactPathBeforeConnecting(t *testing.T) {
	db := newSandboxTestDB(t)
	h := &HTTP{workspaces: &service{db: db}}
	recorder := httptest.NewRecorder()
	h.downloadFile(recorder, httptest.NewRequest(http.MethodGet, "/internal/sandbox/files/download?userId=user-1&sessionId=session-1&path=uploads/source.txt", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func newSandboxTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := createSchema(t.Context(), db); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}
