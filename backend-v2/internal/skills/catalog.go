package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"edith/backend-v2/internal/volume"
	"gopkg.in/yaml.v3"
)

// Catalog 是 Skills 模块对外提供的目录能力。
// 公共 Skills 启动时加载到内存，用户 Skills 通过 Volume.Service 按请求读取摘要。
type Catalog struct {
	skills  []Skill
	volumes *volume.Service
}

// ListSystemSummaries 返回按稳定顺序排列的内置 Skill 摘要。
func (c *Catalog) ListSystemSummaries() []SkillSummary {
	result := make([]SkillSummary, 0, len(c.skills))
	for _, skill := range c.skills {
		result = append(result, SkillSummary{Name: skill.Name, Description: skill.Description})
	}
	return result
}

// ListUserSummaries 返回当前用户 overview.md 中的自定义 Skill 摘要。
// overview.md 不存在时返回空列表，不会创建用户 Volume。
func (c *Catalog) ListUserSummaries(ctx context.Context, userID string) ([]SkillSummary, error) {
	overview, err := c.ReadUserOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	return parseUserOverview(overview), nil
}

// ReadUserOverview 返回当前用户的自定义 Skill 摘要原文。
// 完整 Skill 正文仍由 Agent 通过 Sandbox 按需读取。
func (c *Catalog) ReadUserOverview(ctx context.Context, userID string) (string, error) {
	return c.volumes.ReadUserOverview(ctx, userID)
}

// parseUserOverview 解析 sync_overview.py 生成的 Skill 摘要列表。
// 标题、说明和路径等 Markdown 行不会被当作 Skill 项。
func parseUserOverview(content string) []SkillSummary {
	result := make([]SkillSummary, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- `") {
			continue
		}

		remaining := strings.TrimPrefix(line, "- `")
		endName := strings.IndexByte(remaining, '`')
		if endName <= 0 {
			continue
		}
		name := strings.TrimSpace(remaining[:endName])
		description := strings.TrimSpace(remaining[endName+1:])
		description = strings.TrimSpace(strings.TrimPrefix(description, "："))
		description = strings.TrimSpace(strings.TrimPrefix(description, ":"))
		if name == "" || description == "" {
			continue
		}
		result = append(result, SkillSummary{Name: name, Description: description})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result
}

// loadCatalog 扫描并加载 Skills 文件，返回只读目录。
func loadCatalog(files fs.FS) (*Catalog, error) {
	paths, err := skillPaths(files)
	if err != nil {
		return nil, err
	}

	loaded := make([]Skill, 0, len(paths))
	seenNames := make(map[string]string, len(paths))
	for _, skillPath := range paths {
		skill, err := loadSkill(files, skillPath)
		if err != nil {
			return nil, err
		}
		if previousPath, exists := seenNames[skill.Name]; exists {
			return nil, fmt.Errorf("duplicate skill name %q in %s and %s", skill.Name, previousPath, skillPath)
		}
		seenNames[skill.Name] = skillPath
		loaded = append(loaded, skill)
	}
	return &Catalog{skills: loaded}, nil
}

// skillPaths 找出 system/<skill-name>/SKILL.md，并按路径排序。
func skillPaths(files fs.FS) ([]string, error) {
	var paths []string
	err := fs.WalkDir(files, "system", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path.Base(filePath) != "SKILL.md" {
			return nil
		}
		if path.Dir(path.Dir(filePath)) != "system" {
			return fmt.Errorf("skill file must be directly under system/<skill-name>: %s", filePath)
		}
		paths = append(paths, filePath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan embedded skills: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// loadSkill 解析一个 Skill 的 SKILL.md 和可选 edith.yaml。
func loadSkill(files fs.FS, skillPath string) (Skill, error) {
	data, err := fs.ReadFile(files, skillPath)
	if err != nil {
		return Skill{}, fmt.Errorf("read %s: %w", skillPath, err)
	}
	frontmatter, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Skill{}, fmt.Errorf("parse %s: %w", skillPath, err)
	}

	var metadata skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return Skill{}, fmt.Errorf("parse %s frontmatter: %w", skillPath, err)
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if metadata.Name == "" || metadata.Description == "" {
		return Skill{}, fmt.Errorf("%s frontmatter requires name and description", skillPath)
	}

	edith, err := loadEdithMetadata(files, path.Join(path.Dir(skillPath), "edith.yaml"))
	if err != nil {
		return Skill{}, err
	}
	if edith.DisplayName == "" {
		edith.DisplayName = metadata.Name
	}
	if edith.ShortDescription == "" {
		edith.ShortDescription = metadata.Description
	}

	return Skill{
		ID:               path.Base(path.Dir(skillPath)),
		Name:             metadata.Name,
		Description:      metadata.Description,
		Body:             strings.TrimSpace(body),
		DisplayName:      edith.DisplayName,
		ShortDescription: edith.ShortDescription,
	}, nil
}

// loadEdithMetadata 读取展示元数据；文件不存在时交给调用方回退。
func loadEdithMetadata(files fs.FS, metadataPath string) (edithMetadata, error) {
	data, err := fs.ReadFile(files, metadataPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return edithMetadata{}, nil
		}
		return edithMetadata{}, fmt.Errorf("read %s: %w", metadataPath, err)
	}

	var metadata edithMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return edithMetadata{}, fmt.Errorf("parse %s: %w", metadataPath, err)
	}
	metadata.DisplayName = strings.TrimSpace(metadata.DisplayName)
	metadata.ShortDescription = strings.TrimSpace(metadata.ShortDescription)
	return metadata, nil
}

// splitFrontmatter 分离 SKILL.md 的 YAML 头部和 Markdown 正文。
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "---" {
			continue
		}
		return strings.Join(lines[1:index], "\n"), strings.Join(lines[index+1:], "\n"), nil
	}
	return "", "", fmt.Errorf("SKILL.md frontmatter is not closed")
}
