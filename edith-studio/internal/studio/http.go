package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"edith/studio/internal/commands"
	"edith/studio/internal/engine"
	"edith/studio/internal/mcp"
	"edith/studio/internal/models"
	"edith/studio/internal/project"
	"edith/studio/internal/session"
	"edith/studio/internal/skills"
	"edith/studio/internal/workspace"
)

// maxRequestBodyBytes 需要容纳最多 5 张图片的 base64 与 JSON 开销。
const maxRequestBodyBytes = 80 << 20

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newHandler(appCtx context.Context, workspaceRuntime *workspace.Workspace) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/runs", runHandler(appCtx, workspaceRuntime))
	mux.HandleFunc("POST /api/runs/{requestID}/cancel", cancelHandler(workspaceRuntime))
	mux.HandleFunc("GET /api/commands", listCommandsHandler(workspaceRuntime.Commands))
	mux.HandleFunc("POST /api/commands", commandHandler(workspaceRuntime))
	mux.HandleFunc("GET /api/mcp", listMCPHandler(workspaceRuntime.MCP))
	mux.HandleFunc("GET /api/skills", listSkillsHandler(workspaceRuntime.Skills))
	mux.HandleFunc("GET /api/models", listModelsHandler(workspaceRuntime.Models))
	mux.HandleFunc("GET /api/files", listFilesHandler(workspaceRuntime.Project))
	mux.HandleFunc("GET /api/files/content", readFileHandler(workspaceRuntime.Project))
	mux.HandleFunc("GET /api/sessions", listSessionsHandler(workspaceRuntime.Sessions, workspaceRuntime.WorkspaceID))
	mux.HandleFunc("GET /api/sessions/{sessionID}", getSessionHandler(workspaceRuntime.Sessions, workspaceRuntime.WorkspaceID))
	mux.HandleFunc("GET /api/sessions/{sessionID}/context", getSessionContextHandler(workspaceRuntime.Sessions, workspaceRuntime.WorkspaceID))
	mux.HandleFunc("DELETE /api/sessions/{sessionID}", deleteSessionHandler(workspaceRuntime.Sessions, workspaceRuntime.WorkspaceID))
	return allowLocalWeb(mux)
}

func listModelsHandler(modelModule *models.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if modelModule == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "models_unavailable", "model catalog is unavailable")
			return
		}
		writeJSON(responseWriter, http.StatusOK, modelModule.Catalog())
	}
}

func listMCPHandler(mcpModule *mcp.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if mcpModule == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "mcp_unavailable", "MCP module is unavailable")
			return
		}
		writeJSON(responseWriter, http.StatusOK, struct {
			Servers []mcp.ServerStatus `json:"servers"`
		}{Servers: mcpModule.Status()})
	}
}

func listSkillsHandler(skillsModule *skills.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if skillsModule == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "skills_unavailable", "skills module is unavailable")
			return
		}
		writeJSON(responseWriter, http.StatusOK, struct {
			Skills []skills.Entry `json:"skills"`
		}{Skills: skillsModule.List()})
	}
}

func listCommandsHandler(commandModule *commands.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, _ *http.Request) {
		if commandModule == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "commands_unavailable", "command catalog is unavailable")
			return
		}
		writeJSON(responseWriter, http.StatusOK, struct {
			Commands []commands.Definition `json:"commands"`
		}{Commands: commandModule.List()})
	}
}

func commandHandler(workspaceRuntime *workspace.Workspace) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		input, err := decodeCommandInput(responseWriter, request)
		if err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_command", err.Error())
			return
		}
		if workspaceRuntime.Commands == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "commands_unavailable", "command execution is unavailable")
			return
		}
		if err := workspaceRuntime.Commands.Execute(request.Context(), input); err != nil {
			writeCommandError(responseWriter, err)
			return
		}
		writeJSON(responseWriter, http.StatusOK, commands.Result{
			Command: commandName(input.Command),
			Status:  "completed",
		})
	}
}

func writeCommandError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, commands.ErrInvalidCommand), errors.Is(err, engine.ErrInvalidCompactInput):
		writeJSONError(responseWriter, http.StatusBadRequest, "invalid_command", err.Error())
	case errors.Is(err, commands.ErrUnknownCommand):
		writeJSONError(responseWriter, http.StatusBadRequest, "unknown_command", err.Error())
	case errors.Is(err, engine.ErrSessionBusy):
		writeJSONError(responseWriter, http.StatusConflict, "session_busy", "this session already has an active run")
	case errors.Is(err, models.ErrUnknownModel), errors.Is(err, models.ErrUnsupportedThinkingMode):
		writeJSONError(responseWriter, http.StatusBadRequest, "invalid_model_selection", err.Error())
	case errors.Is(err, session.ErrInvalidSessionID):
		writeJSONError(responseWriter, http.StatusBadRequest, "invalid_session_id", "sessionId is invalid")
	case errors.Is(err, session.ErrSessionNotFound):
		writeJSONError(responseWriter, http.StatusNotFound, "session_not_found", "the requested session does not exist")
	default:
		writeJSONError(responseWriter, http.StatusInternalServerError, "command_failed", "unable to execute command")
	}
}

func commandName(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(fields[0]), "/")
}

func listSessionsHandler(sessionModule *session.Module, workspaceID string) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		sessions, err := sessionModule.List(request.Context(), workspaceID)
		if err != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "sessions_list_failed", "unable to list sessions")
			return
		}
		writeJSON(responseWriter, http.StatusOK, struct {
			Sessions []session.Summary `json:"sessions"`
		}{Sessions: sessions})
	}
}

func getSessionHandler(sessionModule *session.Module, workspaceID string) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		sessionID, err := requestSessionID(request)
		if err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_session_id", "sessionId is invalid")
			return
		}
		history, err := sessionModule.Get(request.Context(), workspaceID, sessionID)
		if err != nil {
			writeSessionError(responseWriter, err)
			return
		}
		writeJSON(responseWriter, http.StatusOK, history)
	}
}

func deleteSessionHandler(sessionModule *session.Module, workspaceID string) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		sessionID, err := requestSessionID(request)
		if err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_session_id", "sessionId is invalid")
			return
		}
		if err := sessionModule.Delete(request.Context(), workspaceID, sessionID); err != nil {
			writeSessionError(responseWriter, err)
			return
		}
		responseWriter.WriteHeader(http.StatusNoContent)
	}
}

func getSessionContextHandler(sessionModule *session.Module, workspaceID string) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		sessionID, err := requestSessionID(request)
		if err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_session_id", "sessionId is invalid")
			return
		}
		usage, err := sessionModule.ContextUsage(request.Context(), workspaceID, sessionID)
		if err != nil {
			writeSessionError(responseWriter, err)
			return
		}
		writeJSON(responseWriter, http.StatusOK, usage)
	}
}

func requestSessionID(request *http.Request) (string, error) {
	sessionID, err := urlPathValue(request, "sessionID")
	if err != nil || strings.TrimSpace(sessionID) == "" || strings.ContainsAny(sessionID, `/\\`) {
		return "", session.ErrInvalidSessionID
	}
	return sessionID, nil
}

func urlPathValue(request *http.Request, name string) (string, error) {
	return url.PathUnescape(request.PathValue(name))
}

func writeSessionError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrInvalidSessionID):
		writeJSONError(responseWriter, http.StatusBadRequest, "invalid_session_id", "sessionId is invalid")
	case errors.Is(err, session.ErrSessionNotFound):
		writeJSONError(responseWriter, http.StatusNotFound, "session_not_found", "the requested session does not exist")
	default:
		writeJSONError(responseWriter, http.StatusInternalServerError, "session_read_failed", "unable to read the requested session")
	}
}

// allowLocalWeb 只允许本机开发界面直接访问 Agent API。
func allowLocalWeb(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "http://127.0.0.1:3000" || origin == "http://localhost:3000" {
			responseWriter.Header().Set("Access-Control-Allow-Origin", origin)
			responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			responseWriter.Header().Set("Vary", "Origin")
		}
		if request.Method == http.MethodOptions {
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(responseWriter, request)
	})
}

func runHandler(appCtx context.Context, workspaceRuntime *workspace.Workspace) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		input, err := decodeRunInput(responseWriter, request)
		if err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := engine.ValidateInput(input); err != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_request", "requestId, sessionId and message are required")
			return
		}

		flusher, ok := responseWriter.(http.Flusher)
		if !ok {
			writeJSONError(responseWriter, http.StatusInternalServerError, "stream_unsupported", "HTTP streaming is not supported")
			return
		}

		streamStarted := false
		send := func(streamEvent engine.StreamEvent) error {
			if !streamStarted {
				startSSE(responseWriter)
				streamStarted = true
			}
			return writeSSE(responseWriter, flusher, streamEvent)
		}
		err = workspaceRuntime.Engine.Run(appCtx, input, send)
		if errors.Is(err, engine.ErrSessionBusy) && !streamStarted {
			writeJSONError(responseWriter, http.StatusConflict, "session_busy", "this session already has an active run")
			return
		}
		if (errors.Is(err, models.ErrUnknownModel) || errors.Is(err, models.ErrUnsupportedThinkingMode)) && !streamStarted {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_model_selection", err.Error())
			return
		}
		if errors.Is(err, engine.ErrInvalidImage) && !streamStarted {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_image", err.Error())
			return
		}
		if errors.Is(err, engine.ErrModelNotVision) && !streamStarted {
			writeJSONError(responseWriter, http.StatusBadRequest, "model_not_vision", err.Error())
			return
		}
		if err != nil && !streamStarted {
			writeJSONError(responseWriter, http.StatusInternalServerError, "run_failed", "unable to start Agent run")
			return
		}
		if err != nil {
			log.Printf("Agent run %q: %v", input.RequestID, err)
		}
	}
}

func cancelHandler(workspaceRuntime *workspace.Workspace) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.PathValue("requestID"))
		if requestID == "" {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_request", "requestID is required")
			return
		}
		if !workspaceRuntime.Engine.Cancel(requestID) {
			writeJSONError(responseWriter, http.StatusNotFound, "run_not_found", "the run is no longer active")
			return
		}
		responseWriter.WriteHeader(http.StatusNoContent)
	}
}

func listFilesHandler(projectModule *project.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		entries, err := projectModule.ListChildren(request.URL.Query().Get("path"))
		if err != nil {
			writeProjectError(responseWriter, err)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(struct {
			Entries []project.FileEntry `json:"entries"`
		}{Entries: entries})
	}
}

func readFileHandler(projectModule *project.Module) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		content, err := projectModule.ReadText(request.URL.Query().Get("path"))
		if err != nil {
			writeProjectError(responseWriter, err)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(content)
	}
}

func writeProjectError(responseWriter http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, project.ErrInvalidPath),
		errors.Is(err, project.ErrNotDirectory),
		errors.Is(err, project.ErrNotRegularFile),
		errors.Is(err, project.ErrNotTextFile):
		writeJSONError(responseWriter, http.StatusBadRequest, "invalid_path", "the requested project path cannot be read")
	case errors.Is(err, project.ErrPathOutsideRoot):
		writeJSONError(responseWriter, http.StatusForbidden, "path_outside_project", "the requested path is outside the current project")
	case errors.Is(err, os.ErrNotExist):
		writeJSONError(responseWriter, http.StatusNotFound, "file_not_found", "the requested project path does not exist")
	default:
		writeJSONError(responseWriter, http.StatusInternalServerError, "file_read_failed", "unable to read the requested project path")
	}
}

func decodeRunInput(responseWriter http.ResponseWriter, request *http.Request) (engine.RunInput, error) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input engine.RunInput
	if err := decoder.Decode(&input); err != nil {
		return engine.RunInput{}, errors.New("request body must be valid JSON")
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return engine.RunInput{}, errors.New("request body must contain one JSON value")
	}
	return input, nil
}

func decodeCommandInput(responseWriter http.ResponseWriter, request *http.Request) (commands.Input, error) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input commands.Input
	if err := decoder.Decode(&input); err != nil {
		return commands.Input{}, errors.New("request body must be valid JSON")
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return commands.Input{}, errors.New("request body must contain one JSON value")
	}
	return input, nil
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func startSSE(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "text/event-stream")
	responseWriter.Header().Set("Cache-Control", "no-cache")
	responseWriter.Header().Set("Connection", "keep-alive")
	responseWriter.WriteHeader(http.StatusOK)
}

func writeSSE(responseWriter http.ResponseWriter, flusher http.Flusher, streamEvent engine.StreamEvent) error {
	contents, err := json.Marshal(streamEvent)
	if err != nil {
		return fmt.Errorf("encode stream event: %w", err)
	}
	if _, err := fmt.Fprintf(responseWriter, "data: %s\n\n", contents); err != nil {
		return fmt.Errorf("write stream event: %w", err)
	}
	flusher.Flush()
	return nil
}

func writeJSONError(responseWriter http.ResponseWriter, status int, code, message string) {
	writeJSON(responseWriter, status, apiError{Code: code, Message: message})
}

func writeJSON(responseWriter http.ResponseWriter, status int, value any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(value)
}
