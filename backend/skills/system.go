// Package skills 负责读取 EDITH 的系统 Skills。
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSystemOverview 扫描 systemRoot 下的 SKILL.md，生成给 LLM 使用的短概览。
func LoadSystemOverview(systemRoot string) (string, error) {
	entries, err := os.ReadDir(systemRoot)
	if err != nil {
		return "", fmt.Errorf("read system skills directory %q: %w", systemRoot, err)
	}

	var items []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name, description, err := readMetadata(filepath.Join(systemRoot, entry.Name(), "SKILL.md"))
		if err != nil {
			return "", err
		}
		items = append(items, fmt.Sprintf(
			"- %s：%s\n  读取：`skills/system/%s/SKILL.md`",
			name,
			description,
			entry.Name(),
		))
	}

	if len(items) == 0 {
		return "", nil
	}

	sort.Strings(items)
	return "## 可用系统 Skills\n\n" + strings.Join(items, "\n") +
		"\n\n当任务明确匹配某个 Skill 时，直接对该条目的“读取”路径调用一次 file_read。" +
		"不要对 skills/、skills/system/ 或某个 Skill 目录调用 file_read。", nil
}

func readMetadata(path string) (string, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read system skill %q: %w", path, err)
	}

	_, rest, ok := strings.Cut(strings.TrimPrefix(string(content), "\ufeff"), "---\n")
	if !ok {
		return "", "", fmt.Errorf("system skill %q must start with YAML frontmatter", path)
	}
	header, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", fmt.Errorf("system skill %q has unclosed YAML frontmatter", path)
	}

	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(header), &meta); err != nil {
		return "", "", fmt.Errorf("parse system skill metadata %q: %w", path, err)
	}
	if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Description) == "" {
		return "", "", fmt.Errorf("system skill %q requires name and description", path)
	}
	return strings.TrimSpace(meta.Name), strings.TrimSpace(meta.Description), nil
}
