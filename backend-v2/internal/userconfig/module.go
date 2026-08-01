package userconfig

import (
	"database/sql"
	"errors"
)

// Dependencies 是 userconfig 初始化所需的明确外部配置。
type Dependencies struct {
	DB             *sql.DB
	DefaultModelID string
}

// Module 是用户配置模块对外公开的能力集合。
type Module struct {
	Settings  *Settings
	Providers *Providers
	MCP       *MCP
	Bindings  *Bindings
	HTTP      *HTTP
}

// New 在调用方提供的数据库上创建用户配置模块。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("userconfig requires a database")
	}
	if err := createSchema(deps.DB); err != nil {
		return nil, err
	}

	settingsStore := &settingsStore{db: deps.DB}
	providerStore := &providerStore{db: deps.DB}
	mcpStore := &mcpStore{db: deps.DB}
	bindingStore := &bindingStore{db: deps.DB}

	settings := &Settings{store: settingsStore}
	providers := &Providers{store: providerStore}
	mcpServers := &MCP{store: mcpStore}
	bindings := &Bindings{store: bindingStore}
	httpAPI := &HTTP{settings: settings, providers: providers, mcp: mcpServers, defaultModelID: deps.DefaultModelID}

	return &Module{
		Settings:  settings,
		Providers: providers,
		MCP:       mcpServers,
		Bindings:  bindings,
		HTTP:      httpAPI,
	}, nil
}
