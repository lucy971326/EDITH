package webadapter

import (
	"encoding/json"
	"fmt"
	"net/http"

	"edith/backend-v1/internal/gateway"
)

// webMessageRequest 是 Web BFF 发给 WebAdapter 的请求格式。
// UserID 已由 BFF 从 Clerk 注入；浏览器不能自行提供它。
type webMessageRequest struct {
	RequestID         string   `json:"requestId"`
	UserID            string   `json:"userId"`
	SessionID         string   `json:"sessionId"`
	Message           string   `json:"message"`
	ImageIDs          []string `json:"imageIds"`
	ModelID           string   `json:"modelId"`
	ReasoningOptionID string   `json:"reasoningOptionId,omitempty"`
}

// handleStreamMessage 处理 Web BFF 发起的一次流式消息请求。
// 输入：已鉴权的 Gateway MessageRequest JSON。
// 输出：Gateway StreamEvent 被编码为 SSE；浏览器断线后仍读完事件流以保持后台任务完整。
func (s *Server) handleStreamMessage(w http.ResponseWriter, r *http.Request) {
	var request webMessageRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, gateway.APIError{Type: "invalid_request", Message: "invalid gateway message request"})
		return
	}

	stream, apiError := s.agentGateway.Run(gateway.IncomingMessage{
		Channel:           gateway.WebChannel,
		ExternalUserID:    request.UserID,
		SessionKey:        request.SessionID,
		RequestID:         request.RequestID,
		Message:           request.Message,
		ImageIDs:          request.ImageIDs,
		ModelID:           request.ModelID,
		ReasoningOptionID: request.ReasoningOptionID,
	})
	if apiError != nil {
		writeGatewayError(w, apiError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	clientConnected := true
	for item := range stream.Events {
		if !clientConnected {
			continue
		}
		if err := writeSSE(w, item); err != nil {
			clientConnected = false
		}
	}
}

func writeSSE(w http.ResponseWriter, item gateway.StreamEvent) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encode gateway SSE event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("write gateway SSE event: %w", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
