package webadapter

import (
	"errors"
	"net/http"

	"edith/backend-v2/internal/gateway"
)

// Adapter 是 Web 渠道入口，只理解 HTTP、JSON 和 SSE。
type Adapter struct {
	gateway *gateway.Service
}

// New 创建 WebAdapter；Gateway 必须由 main 显式提供。
func New(agentGateway *gateway.Service) (*Adapter, error) {
	if agentGateway == nil {
		return nil, errors.New("webadapter requires a gateway")
	}
	return &Adapter{gateway: agentGateway}, nil
}

// Register 注册 Web BFF 使用的消息、状态和取消路由。
func (a *Adapter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/gateway/messages:stream", a.StreamMessage)
	mux.HandleFunc("GET /internal/gateway/runs/{requestID}", a.RunStatus)
	mux.HandleFunc("POST /internal/gateway/runs/{requestID}/cancel", a.CancelRun)
}
