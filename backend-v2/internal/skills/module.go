package skills

import (
	"embed"
	"fmt"
)

//go:embed system
var systemFiles embed.FS

// Module 是 Skills 模块对外能力的集合。
// 当前只公开 Catalog，未来可在这里增加 HTTP 等独立入口。
type Module struct {
	Catalog *Catalog
}

// New 启动时加载并校验全部内置 Skills。
// 任何 Skill 格式错误都会阻止服务启动，避免运行中出现隐式缺失。
func New() (*Module, error) {
	catalog, err := loadCatalog(systemFiles)
	if err != nil {
		return nil, fmt.Errorf("load built-in skills: %w", err)
	}
	return &Module{Catalog: catalog}, nil
}
