// Package skills 提供内置 Skills 的加载与目录能力。
package skills

const (
	// SystemPath 是 Sandbox 文件工具使用的公共 Skills 相对路径。
	SystemPath = "skills/system"
	// CustomPath 是未来挂载用户 Skills Volume 的相对路径。
	CustomPath = "skills/custom"
)

// Skill 是一个已经解析完成的 Skill。
// Body 供未来需要注入完整 Skill 正文时使用；当前 AgentRun 只使用摘要。
type Skill struct {
	ID               string
	Name             string
	Description      string
	Body             string
	DisplayName      string
	ShortDescription string
}

// SkillSummary 是传给 AgentRun 的最小 Skill 描述。
// 当前只把名称和说明注入 RunOptions，不把完整正文放进每次请求。
type SkillSummary struct {
	Name        string
	Description string
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type edithMetadata struct {
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
}
