// Package gateway exposes EDITH's channel-neutral Agent message protocol.
package gateway

import "edith/backend-v1/internal/usage"

// MessageRequest is one trusted request to run EDITH for a user session.
// Web's BFF and future channel adapters authenticate the user before calling
// the gateway.
type MessageRequest struct {
	RequestID         string   `json:"requestId"`
	UserID            string   `json:"userId"`
	SessionID         string   `json:"sessionId"`
	Message           string   `json:"message"`
	ImageIDs          []string `json:"imageIds"`
	ModelID           string   `json:"modelId"`
	ReasoningOptionID string   `json:"reasoningOptionId,omitempty"`
}

type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamEvent describes Agent progress without choosing a browser, IM card,
// or GitHub rendering. Every channel projects these events into its own UI.
type StreamEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
	RequestID string `json:"requestId,omitempty"`

	AssistantID string `json:"assistantId,omitempty"`
	BlockID     string `json:"blockId,omitempty"`
	BlockType   string `json:"blockType,omitempty"`
	Delta       string `json:"delta,omitempty"`

	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	ToolStatus string `json:"toolStatus,omitempty"`
	ToolResult string `json:"toolResult,omitempty"`

	Usage *usage.Summary `json:"sessionUsage,omitempty"`
	Error *APIError      `json:"error,omitempty"`
}

type RunStatusResponse struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
}
