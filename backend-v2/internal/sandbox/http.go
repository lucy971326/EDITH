package sandbox

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"edith/backend-v2/internal/httpx"
	"github.com/eric642/e2b-go-sdk"
)

// HTTP 是 Sandbox 对 Web BFF 提供的只读文件浏览能力。
type HTTP struct{ workspaces *service }

// listFiles 返回已存在 Sandbox 中目录的直接子项。
func (h *HTTP) listFiles(writer http.ResponseWriter, request *http.Request) {
	workspace, directory, ok := h.existingWorkspace(writer, request, true)
	if !ok {
		return
	}
	entries, err := workspace.Files.List(request.Context(), directory, e2b.FsOptions{Depth: 1})
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot list sandbox files")
		return
	}
	result := FileTreeResponse{Path: relativePath(directory), Entries: make([]FileEntry, 0, len(entries))}
	for _, entry := range entries {
		kind := "file"
		if entry.Type == e2b.EntryTypeDirectory {
			kind = "directory"
		}
		result.Entries = append(result.Entries, FileEntry{Name: entry.Name, Path: relativePath(entry.Path), Type: kind, Size: entry.Size})
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

// readContent 返回最多 32 KiB 的 UTF-8 文本预览；二进制文件不在此接口暴露。
func (h *HTTP) readContent(writer http.ResponseWriter, request *http.Request) {
	workspace, filePath, ok := h.existingWorkspace(writer, request, false)
	if !ok {
		return
	}
	info, err := workspace.Files.Stat(request.Context(), filePath, e2b.FsOptions{})
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot read sandbox file")
		return
	}
	if info.Type == e2b.EntryTypeDirectory {
		httpx.WriteError(writer, http.StatusBadRequest, "not_a_file", "path must identify a file")
		return
	}
	reader, err := workspace.Files.ReadStream(request.Context(), filePath, e2b.FsOptions{})
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot read sandbox file")
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, defaultReadLimit+1))
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot read sandbox file")
		return
	}
	truncated := len(data) > defaultReadLimit
	if truncated {
		data = data[:defaultReadLimit]
	}
	if !isPreviewableText(data) {
		httpx.WriteError(writer, http.StatusUnprocessableEntity, "file_not_previewable", "only UTF-8 text files can be previewed")
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, FileContentResponse{Path: relativePath(filePath), Content: string(data), Truncated: truncated})
}

// existingWorkspace 统一验证 HTTP 参数和路径，并确保 HTTP 永远不会触发资源创建。
func (h *HTTP) existingWorkspace(writer http.ResponseWriter, request *http.Request, allowRoot bool) (*e2b.Sandbox, string, bool) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	sessionID := strings.TrimSpace(request.URL.Query().Get("sessionId"))
	if userID == "" || sessionID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and sessionId are required")
		return nil, "", false
	}
	filePath, err := workspacePath(request.URL.Query().Get("path"), allowRoot)
	if err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_path", err.Error())
		return nil, "", false
	}
	workspace, err := h.workspaces.ExistingWorkspace(request.Context(), userID, sessionID)
	if errors.Is(err, errSandboxNotFound) {
		httpx.WriteError(writer, http.StatusNotFound, "sandbox_not_found", "sandbox has not been created for this session")
		return nil, "", false
	}
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_unavailable", "cannot connect to sandbox")
		return nil, "", false
	}
	return workspace, filePath, true
}

func isPreviewableText(data []byte) bool {
	return utf8.Valid(data) && !bytes.Contains(data, []byte{0})
}
