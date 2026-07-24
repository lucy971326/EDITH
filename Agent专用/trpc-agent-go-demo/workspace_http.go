package main

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"demo/sandbox"
)

const maxDownloadBytes = 50 << 20 // 50 MiB

type WorkspaceListing struct {
	Path    string              `json:"path"`
	Entries []sandbox.FileEntry `json:"entries"`
}

// workspaceListHandler 返回当前会话工作区某个目录的直接子项。
// 前端只认识虚拟工作区根目录，不会接触 Local/E2B 的真实绝对路径。
func workspaceListHandler(provider sandbox.BackendProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		backend, workspacePath, err := workspaceBackend(req, provider, true)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}
		entries, err := backend.ListDir(req.Context(), workspacePath, 1)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, "list workspace: "+err.Error()), http.StatusInternalServerError)
			return
		}
		// Local 与 E2B 的底层 Path 表示可能不同；对 Web 统一暴露虚拟相对路径。
		visibleEntries := make([]sandbox.FileEntry, 0, len(entries))
		for _, entry := range entries {
			// 点开工作区只展示用户与 Agent 文件，不暴露 Sandbox 环境配置。
			if strings.HasPrefix(entry.Name, ".") {
				continue
			}
			entry.Path = path.Join(workspacePath, entry.Name)
			visibleEntries = append(visibleEntries, entry)
		}
		writeJSON(w, WorkspaceListing{Path: workspacePath, Entries: visibleEntries})
	}
}

// workspaceDownloadHandler 将当前用户当前会话中的一个文件作为浏览器下载返回。
func workspaceDownloadHandler(provider sandbox.BackendProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		backend, workspacePath, err := workspaceBackend(req, provider, false)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}
		data, err := backend.DownloadFile(req.Context(), workspacePath)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, "download workspace file: "+err.Error()), http.StatusNotFound)
			return
		}
		if len(data) > maxDownloadBytes {
			http.Error(w, `{"error":"file exceeds 50 MiB download limit"}`, http.StatusRequestEntityTooLarge)
			return
		}

		name := path.Base(workspacePath)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	}
}

func workspaceBackend(req *http.Request, provider sandbox.BackendProvider, allowRoot bool) (sandbox.ExecBackend, string, error) {
	userID := strings.TrimSpace(req.URL.Query().Get("user_id"))
	sessionID := strings.TrimSpace(req.URL.Query().Get("session_id"))
	if userID == "" || sessionID == "" {
		return nil, "", fmt.Errorf("bad request: user_id and session_id are required")
	}
	workspacePath, err := cleanWorkspacePath(req.URL.Query().Get("path"), allowRoot)
	if err != nil {
		return nil, "", err
	}
	backend, err := provider.GetBackend(req.Context(), sandbox.WorkspaceID{UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, "", fmt.Errorf("open workspace: %w", err)
	}
	return backend, workspacePath, nil
}

func cleanWorkspacePath(raw string, allowRoot bool) (string, error) {
	p := strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/")
	if p == "" || p == "." {
		if allowRoot {
			return ".", nil
		}
		return "", fmt.Errorf("bad request: file path is required")
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("bad request: absolute paths are not allowed")
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", fmt.Errorf("bad request: '..' is not allowed")
		}
	}
	cleaned := path.Clean(p)
	if cleaned == "." {
		return "", fmt.Errorf("bad request: file path is required")
	}
	return cleaned, nil
}

func writeWorkspaceError(w http.ResponseWriter, err error) {
	if strings.HasPrefix(err.Error(), "bad request:") {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, strings.TrimPrefix(err.Error(), "bad request: ")), http.StatusBadRequest)
		return
	}
	http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
}
