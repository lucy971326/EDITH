// Package images 管理聊天图片的元数据、COS 访问和 Agent 图片输入。
package images

import (
	"context"
	"database/sql"
	"errors"
)

// Config 是仅由服务端持有的 COS 配置。
type Config struct {
	Bucket    string
	Region    string
	SecretID  string
	SecretKey string
}

// Dependencies 是创建图片模块需要的长期依赖。
type Dependencies struct {
	DB     *sql.DB
	Config Config
}

// Module 是图片模块对外提供的能力集合。
type Module struct {
	AgentInput    *AgentInput
	SessionImages *SessionImages
	HTTP          *HTTP
}

// New 创建图片模块；数据库由调用方持有，COS 客户端由模块持有。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("image database is required")
	}
	config := normalizeConfig(deps.Config)
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	cos, err := newCOSClient(config) // cos的能力主要是控制腾讯云服务器上面的COS服务，本质上是Client
	if err != nil {
		return nil, err
	}
	store := &store{db: deps.DB}
	if err := store.createSchema(context.Background()); err != nil {
		return nil, err
	}
	service := &service{store: store, cos: cos} // service组合了COSClient+DB的能力
	input := &AgentInput{service: service}
	return &Module{
		AgentInput:    input,
		SessionImages: &SessionImages{service: service},
		HTTP:          &HTTP{service: service},
	}, nil
}
