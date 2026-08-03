package sandbox

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"edith/backend-v2/internal/httpx"
	"github.com/eric642/e2b-go-sdk"
)

// HTTP 是 Sandbox 对 Web BFF 提供的文件浏览、上传与交付下载能力。
type HTTP struct{ workspaces *service }

const maxUploadSize = 50 << 20

// uploadFile 流式写入当前用户会话的 uploads/，首次上传允许创建工作区。
func (h *HTTP) uploadFile(writer http.ResponseWriter, request *http.Request) {
	userID, sessionID := strings.TrimSpace(request.URL.Query().Get("userId")), strings.TrimSpace(request.URL.Query().Get("sessionId"))
	if userID == "" || sessionID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId and sessionId are required")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxUploadSize+1024*1024)
	if err := request.ParseMultipartForm(1024 * 1024); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_upload", "invalid or oversized multipart upload")
		return
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_upload", "file is required")
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxUploadSize {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_upload", "file must be non-empty and at most 50 MB")
		return
	}
	name := path.Base(strings.ReplaceAll(header.Filename, `\\`, "/"))
	if name == "." || name == "/" || strings.TrimSpace(name) == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_upload", "invalid file name")
		return
	}
	workspace, err := h.workspaces.Workspace(request.Context(), userID, sessionID)
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_unavailable", "cannot create sandbox workspace")
		return
	}
	target, err := h.availableUploadPath(request, workspace, name)
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot prepare sandbox upload path")
		return
	}
	if _, err := workspace.Files.Write(request.Context(), target, io.LimitReader(file, maxUploadSize+1), e2b.FsOptions{}); err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot upload sandbox file")
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, FileUploadResponse{Path: relativePath(target), Name: path.Base(target), Size: header.Size})
}

// availableUploadPath 在 uploads/ 中保留原始名称；同名时生成可读副本名。
func (h *HTTP) availableUploadPath(request *http.Request, workspace *e2b.Sandbox, name string) (string, error) {
	base, ext := strings.TrimSuffix(name, path.Ext(name)), path.Ext(name)
	for index := 1; ; index++ {
		candidate := name
		if index > 1 {
			candidate = base + " (" + strconv.Itoa(index) + ")" + ext
		}
		target := path.Join(Workspace.Root, Workspace.Uploads, candidate)
		exists, err := workspace.Files.Exists(request.Context(), target, e2b.FsOptions{})
		if err != nil {
			return "", err
		}
		if !exists {
			return target, nil
		}
	}
}

// downloadFile 仅流式返回 artifacts/ 内已存在的普通交付文件。
func (h *HTTP) downloadFile(writer http.ResponseWriter, request *http.Request) {
	requestedPath, err := workspacePath(request.URL.Query().Get("path"), false)
	if err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_path", err.Error())
		return
	}
	if !strings.HasPrefix(relativePath(requestedPath), Workspace.Artifacts+"/") {
		httpx.WriteError(writer, http.StatusForbidden, "download_not_allowed", "only artifacts files can be downloaded")
		return
	}
	workspace, relative, ok := h.existingWorkspace(writer, request, false)
	if !ok {
		return
	}
	info, err := workspace.Files.Stat(request.Context(), relative, e2b.FsOptions{})
	if err != nil || info.Type == e2b.EntryTypeDirectory {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_download", "path must identify an artifact file")
		return
	}
	reader, err := workspace.Files.ReadStream(request.Context(), relative, e2b.FsOptions{})
	if err != nil {
		httpx.WriteError(writer, http.StatusBadGateway, "sandbox_files_unavailable", "cannot download sandbox file")
		return
	}
	defer reader.Close()
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(relative)}))
	if info.Size > 0 {
		writer.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	_, _ = io.Copy(writer, reader)
}

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
