package webadapter

import (
	"encoding/json"
	"fmt"
	"net/http"

	"edith/backend-v2/internal/gateway"
	"edith/backend-v2/internal/httpx"
)

// StreamMessage 读取一条 Web 消息，并把 Gateway 中性事件写成 SSE。
func (a *Adapter) StreamMessage(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	defer request.Body.Close()
	var input MessageRequest
	if err := httpx.ReadJSON(request, &input); err != nil {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "invalid gateway message request")
		return
	}

	stream, runError := a.gateway.Run(gateway.IncomingMessage{
		Channel: gateway.WebChannel, ExternalUserID: input.UserID, SessionKey: input.SessionID,
		RequestID: input.RequestID, Message: input.Message, ImageIDs: input.ImageIDs,
		UploadPaths: input.UploadPaths,
		ModelID:     input.ModelID, ReasoningOptionID: input.ReasoningOptionID,
	})
	if runError != nil {
		writeGatewayError(writer, runError)
		return
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("X-Accel-Buffering", "no")
	clientConnected := true
	for streamEvent := range stream.Events {
		if clientConnected && writeSSE(writer, streamEvent) != nil {
			clientConnected = false
		}
	}
}

func writeSSE(writer http.ResponseWriter, streamEvent gateway.Event) error {
	data, err := json.Marshal(streamEvent)
	if err != nil {
		return fmt.Errorf("encode SSE event: %w", err)
	}
	if _, err := fmt.Fprintf(writer, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("write SSE event: %w", err)
	}
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
