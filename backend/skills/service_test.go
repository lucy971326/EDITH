package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSystemOverview(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "pdf", "处理 PDF 文件")
	writeSkill(t, root, "spreadsheet", "处理 Excel 文件")

	overview, err := LoadSystemOverview(root)
	if err != nil {
		t.Fatalf("LoadSystemOverview() error = %v", err)
	}

	if !strings.Contains(overview, "- pdf：处理 PDF 文件") {
		t.Fatalf("overview misses pdf: %q", overview)
	}
	if !strings.Contains(overview, "读取：`skills/system/pdf/SKILL.md`") {
		t.Fatalf("overview misses full skill path: %q", overview)
	}
	if !strings.Contains(overview, "不要对 skills/、skills/system/ 或某个 Skill 目录调用 file_read") {
		t.Fatalf("overview misses direct-read rule: %q", overview)
	}
	if strings.Index(overview, "pdf") > strings.Index(overview, "spreadsheet") {
		t.Fatalf("overview is not sorted: %q", overview)
	}
}

func TestProjectSystemSkills(t *testing.T) {
	overview, err := LoadSystemOverview("system")
	if err != nil {
		t.Fatalf("LoadSystemOverview(system) error = %v", err)
	}
	if overview == "" || !strings.Contains(overview, "读取：`skills/system/") {
		t.Fatalf("overview misses project skill read path: %q", overview)
	}
}

func writeSkill(t *testing.T, root, name, description string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n说明"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
