// Package conversation 把框架会话历史投影为浏览器 Timeline。
package conversation

import (
	"errors"
	"strings"

	"edith/backend-v2/internal/usage"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
)

// Dependencies 是创建会话模块需要的长期依赖。
type Dependencies struct {
	AppName  string
	Sessions frameworksession.Service
	Usage    *usage.Reader
}

// Module 是会话模块对外提供的能力集合。
type Module struct {
	HTTP *HTTP
}

// New 创建会话模块；会话服务和用量读取器都由调用方持有。
func New(deps Dependencies) (*Module, error) {
	if strings.TrimSpace(deps.AppName) == "" {
		return nil, errors.New("conversation app name is required")
	}
	if deps.Sessions == nil {
		return nil, errors.New("conversation session service is required")
	}
	if deps.Usage == nil {
		return nil, errors.New("conversation usage reader is required")
	}
	history := &history{appName: strings.TrimSpace(deps.AppName), sessions: deps.Sessions, usage: deps.Usage}
	return &Module{HTTP: &HTTP{history: history}}, nil
}
