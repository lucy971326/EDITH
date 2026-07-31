// Package webadapter 将 Web BFF 的 HTTP/SSE 协议适配为 Gateway 的进程内调用。
package webadapter

import (
	"errors"
	"net/http"

	"edith/backend-v1/internal/gateway"
)

// Server 是 Web 渠道的适配器。
// 它只理解 HTTP、JSON 与 SSE；Agent 的配置、运行和控制全部委托给 Gateway。
type Server struct {
	agentGateway *gateway.Gateway
}

func New(agentGateway *gateway.Gateway) (*Server, error) {
	if agentGateway == nil {
		return nil, errors.New("web adapter gateway is required")
	}
	return &Server{agentGateway: agentGateway}, nil
}

// Register 注册仅供 Web BFF 调用的 Agent HTTP 接口。
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/gateway/messages:stream", s.handleStreamMessage)
	mux.HandleFunc("GET /internal/gateway/runs/{requestID}", s.handleRunStatus)
	mux.HandleFunc("POST /internal/gateway/runs/{requestID}/cancel", s.handleCancel)
}
