package volume

const (
	// CustomSkillsPath 是用户 Volume 在 Sandbox 中的固定挂载路径。
	CustomSkillsPath = "/home/user/skills/custom"
	// UserOverviewPath 是用户 Volume 中 Skills 摘要索引的路径。
	UserOverviewPath = "/overview.md"
)

// UserVolume 是后端内部使用的用户 Volume 基本信息。
// Token 不放在这个结构体中，避免被调用方意外带到 HTTP 输出。
type UserVolume struct {
	ID   string
	Name string
}

// Mount 是 Sandbox 创建时需要的 Volume 挂载信息。
type Mount struct {
	Name string
	Path string
}

type record struct {
	UserID string
	ID     string
	Name   string
	Token  string
}
