package gateway

import (
	"database/sql"
	"errors"
	"net/http"

	"edith/backend-v1/internal/images"
	"edith/backend-v1/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// Server 持有 Gateway 的长期执行能力。
// 输入中的消息、会话、用户等单次 Run 数据不放在这里，而是只存在于 MessageRequest
// 和 message.go 的局部变量中，避免不同请求共享临时状态。
type Server struct {
	runner          runner.ManagedRunner
	users           *userconfig.Store
	images          *images.Service
	usageDB         *sql.DB
	lanes           *sessionLanes
	userCancelMarks *userCancelMarks
}

func New(
	runner runner.ManagedRunner,
	users *userconfig.Store,
	images *images.Service,
	usageDB *sql.DB,
) (*Server, error) {
	if runner == nil {
		return nil, errors.New("gateway runner is required")
	}
	if users == nil {
		return nil, errors.New("gateway user config store is required")
	}
	if images == nil {
		return nil, errors.New("gateway image service is required")
	}
	if usageDB == nil {
		return nil, errors.New("gateway usage database is required")
	}
	return &Server{
		runner:          runner,
		users:           users,
		images:          images,
		usageDB:         usageDB,
		lanes:           newSessionLanes(),
		userCancelMarks: newUserCancelMarks(),
	}, nil
}

// Register 将 Gateway 的三个 HTTP 接口注册到应用路由：
// 消息执行、活跃状态查询、主动取消。
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/gateway/messages:stream", s.handleStreamMessage)
	mux.HandleFunc("GET /internal/gateway/runs/{requestID}", s.handleRunStatus)
	mux.HandleFunc("POST /internal/gateway/runs/{requestID}/cancel", s.handleCancel)
}
