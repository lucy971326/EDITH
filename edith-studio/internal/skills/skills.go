// Package skills 管理系统级、用户级与项目级 Skills 目录的发现与运行时仓库。
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/skill"
)

// Dependencies 是创建 Skills 模块所需的稳定项目配置。
type Dependencies struct {
	// ProjectRoot 是本次 Studio 进程服务的项目根目录，用于读取项目级 skills 目录。
	ProjectRoot string
	// UserSkillsDir 可覆盖用户级 skills 目录（默认 ~/.edith/skills）。
	// 主要供测试注入；生产留空走默认。
	UserSkillsDir string
	// SystemSkillsDir 可覆盖系统级 skills 目录（默认 ~/.edith/.system-skills）。
	// 独立于用户级目录（~/.edith/skills），避免框架递归扫描把系统技能重复计入用户级。
	// 主要供测试注入；生产留空走默认。
	SystemSkillsDir string
}

// Entry 是暴露给 Studio 和 Web 的单个技能条目。
type Entry struct {
	// Name 是技能名（SKILL.md front matter 的 name，缺省为目录名）。
	Name string `json:"name"`
	// Description 是技能的一句话描述。
	Description string `json:"description"`
	// Level 是技能所在层级：system / user / project。
	Level string `json:"level"`
}

// Module 持有三级 Skills 的发现结果与运行时有效仓库。
type Module struct {
	// entries 是按层级累积的全部技能列表（同名不覆盖、各标层级），供 Web 展示。
	entries []Entry
	// repo 是合并后的运行时仓库（同名技能覆盖：项目 > 用户 > 系统），供 Agent 使用。
	repo skill.Repository
}

// New 扫描三级 skills 目录并构建运行时仓库；任一目录不存在时按空处理，不报错。
func New(dependencies Dependencies) (*Module, error) {
	projectRoot := filepath.Clean(dependencies.ProjectRoot)
	projectSkills := filepath.Join(projectRoot, ".edith", "skills")
	userSkills := defaultOrOverride(userSkillsDir(), dependencies.UserSkillsDir)
	systemSkills := defaultOrOverride(systemSkillsDir(), dependencies.SystemSkillsDir)

	// 启动时先创建用户级与项目级技能目录再扫描（系统级由 seed 全量物化创建）。
	// 创建失败不阻塞：目录不存在时 scanLevel 按空处理，与 MCP 容错一致。
	ensureDir(userSkills)
	ensureDir(projectSkills)

	levels := []struct {
		name string
		path string
	}{
		{"system", systemSkills},
		{"user", userSkills},
		{"project", projectSkills},
	}
	module := &Module{}
	for _, level := range levels {
		entries, err := scanLevel(level.path, level.name)
		if err != nil {
			return nil, fmt.Errorf("scan %s skills: %w", level.name, err)
		}
		module.entries = append(module.entries, entries...)
	}

	// 合并仓库的根顺序即覆盖优先级：项目级在前、用户级次之、系统级最后，同名先到先得。
	repo, err := skill.NewFSRepository(projectSkills, userSkills, systemSkills)
	if err != nil {
		return nil, fmt.Errorf("create skill repository: %w", err)
	}
	module.repo = repo
	return module, nil
}

// scanLevel 读取一个目录下的全部技能概览；目录不存在时返回空列表。
func scanLevel(root string, level string) ([]Entry, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	info, err := os.Stat(root)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	levelRepo, err := skill.NewFSRepository(root)
	if err != nil {
		return nil, err
	}
	summaries := levelRepo.Summaries()
	entries := make([]Entry, 0, len(summaries))
	for _, summary := range summaries {
		entries = append(entries, Entry{
			Name:        summary.Name,
			Description: summary.Description,
			Level:       level,
		})
	}
	return entries, nil
}

// List 返回三级累积的技能列表，供 Studio 和 Web 展示。
func (m *Module) List() []Entry {
	return m.entries
}

// Repository 返回合并后的运行时技能仓库，供 Workspace 传入 llmagent.WithSkills。
func (m *Module) Repository() skill.Repository {
	return m.repo
}

func userSkillsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".edith", "skills")
}

// systemSkillsDir 返回系统级技能目录：~/.edith/.system-skills。
// 内容由 SeedSystemSkills 从二进制内嵌资源全量物化；这里只负责定位。
func systemSkillsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".edith", ".system-skills")
}

func defaultOrOverride(fallback, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return fallback
}

// ensureDir 确保技能目录存在，不存在则创建（0o755）；已存在时无操作。
// 创建失败返回 error，由调用方决定是否忽略（与 MCP 容错一致）。
func ensureDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}
