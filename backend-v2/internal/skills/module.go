package skills

import (
	"embed"
	"errors"
	"fmt"

	"edith/backend-v2/internal/volume"
)

//go:embed system
var systemFiles embed.FS

// Module 是 Skills 模块对外能力的集合。
type Module struct {
	Catalog *Catalog
	HTTP    *HTTP
}

// Dependencies 是创建 Skills 模块需要的其他模块能力。
type Dependencies struct {
	Volumes *volume.Service
}

// New 启动时加载并校验全部内置 Skills。
// 任何 Skill 格式错误都会阻止服务启动，避免运行中出现隐式缺失。
func New(deps Dependencies) (*Module, error) {
	if deps.Volumes == nil {
		return nil, errors.New("skills requires volume service")
	}
	catalog, err := loadCatalog(systemFiles)
	if err != nil {
		return nil, fmt.Errorf("load built-in skills: %w", err)
	}
	catalog.volumes = deps.Volumes
	return &Module{Catalog: catalog, HTTP: &HTTP{catalog: catalog}}, nil
}
