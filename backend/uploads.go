package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"demo/sandbox"

	"github.com/google/uuid"
)

const maxUploadBytes = 20 << 20 // 20 MiB

// UploadedFile 是上传成功后交给前端、再写进聊天消息的文件信息。
type UploadedFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// uploadHandler 是用户上传文件的 Web 入口：
// 读取 multipart 文件 → 找到当前会话工作区 → 写入 Local 或 E2B。
func uploadHandler(provider sandbox.BackendProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxUploadBytes+(1<<20))
		if err := req.ParseMultipartForm(maxUploadBytes + (1 << 20)); err != nil {
			http.Error(w, `{"error":"file is too large or invalid multipart form"}`, http.StatusBadRequest)
			return
		}

		userID := strings.TrimSpace(req.FormValue("user_id"))
		sessionID := strings.TrimSpace(req.FormValue("session_id"))
		if userID == "" || sessionID == "" {
			http.Error(w, `{"error":"user_id and session_id are required"}`, http.StatusBadRequest)
			return
		}

		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"file is required"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
		if err != nil {
			http.Error(w, `{"error":"read upload file"}`, http.StatusBadRequest)
			return
		}
		if len(data) > maxUploadBytes {
			http.Error(w, `{"error":"file exceeds 20 MiB limit"}`, http.StatusRequestEntityTooLarge)
			return
		}

		name := safeUploadName(header.Filename)
		attachmentID := uuid.NewString()
		path := "uploads/" + attachmentID + "/" + name

		backend, err := provider.GetBackend(req.Context(), sandbox.WorkspaceID{
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, "open workspace: "+err.Error()), http.StatusInternalServerError)
			return
		}
		if err := backend.WriteFile(req.Context(), path, data); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, "write upload: "+err.Error()), http.StatusInternalServerError)
			return
		}

		writeJSON(w, UploadedFile{ID: attachmentID, Name: name, Path: path, Size: int64(len(data))})
	}
}

func safeUploadName(raw string) string {
	name := filepath.Base(strings.ReplaceAll(raw, "/", `\`))
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-()[] ", r) {
			return r
		}
		return '_'
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" {
		return "upload.bin"
	}
	return name
}
