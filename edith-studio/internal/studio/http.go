package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"edith/studio/internal/engine"
)

const maxRequestBodyBytes = 1 << 20

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newHandler(appCtx context.Context, engineRuntime *engine.Engine) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/runs", runHandler(appCtx, engineRuntime))
	mux.HandleFunc("POST /api/runs/{requestID}/cancel", cancelHandler(engineRuntime))
	return allowLocalWeb(mux)
}

// allowLocalWeb 只允许本机开发界面直接访问 Agent API。
func allowLocalWeb(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "http://127.0.0.1:3000" || origin == "http://localhost:3000" {
			responseWriter.Header().Set("Access-Control-Allow-Origin", origin)
			responseWriter.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
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

func runHandler(appCtx context.Context, engineRuntime *engine.Engine) http.HandlerFunc {
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
		err = engineRuntime.Run(appCtx, input, send)
		if errors.Is(err, engine.ErrSessionBusy) && !streamStarted {
			writeJSONError(responseWriter, http.StatusConflict, "session_busy", "this session already has an active run")
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

func cancelHandler(engineRuntime *engine.Engine) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.PathValue("requestID"))
		if requestID == "" {
			writeJSONError(responseWriter, http.StatusBadRequest, "invalid_request", "requestID is required")
			return
		}
		if !engineRuntime.Cancel(requestID) {
			writeJSONError(responseWriter, http.StatusNotFound, "run_not_found", "the run is no longer active")
			return
		}
		responseWriter.WriteHeader(http.StatusNoContent)
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
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(apiError{Code: code, Message: message})
}
