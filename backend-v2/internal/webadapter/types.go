// Package webadapter 把 Web BFF 的 HTTP/SSE 协议适配为 Gateway 调用。
package webadapter

// MessageRequest 是 Web BFF 发给后端的流式消息契约。
// UserID 必须由 BFF 从 Clerk 注入，浏览器不能自行提供。
type MessageRequest struct {
	RequestID         string   `json:"requestId"`
	UserID            string   `json:"userId"`
	SessionID         string   `json:"sessionId"`
	Message           string   `json:"message"`
	ImageIDs          []string `json:"imageIds"`
	UploadPaths       []string `json:"uploadPaths"`
	ModelID           string   `json:"modelId"`
	ReasoningOptionID string   `json:"reasoningOptionId,omitempty"`
}
