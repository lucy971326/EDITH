package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, root, dir, name, description string) {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", skillDir, err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func newTestModule(t *testing.T) (*Module, string) {
	t.Helper()
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	systemRoot := t.TempDir()

	// project 与 user 各有一个同名 shared；project 应覆盖 user。
	writeSkill(t, projectRoot, filepath.Join(".edith", "skills", "proj-skill"), "proj-skill", "project skill")
	writeSkill(t, projectRoot, filepath.Join(".edith", "skills", "shared"), "shared", "project shared")
	writeSkill(t, userRoot, "shared", "shared", "user shared")
	writeSkill(t, userRoot, "user-skill", "user-skill", "user skill")
	writeSkill(t, systemRoot, "sys-skill", "sys-skill", "system skill")

	module, err := New(Dependencies{
		ProjectRoot:    projectRoot,
		UserSkillsDir:  userRoot,
		SystemSkillsDir: systemRoot,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return module, projectRoot
}

func TestNewListsAllLevelsCumulatively(t *testing.T) {
	module, _ := newTestModule(t)

	entries := module.List()
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries (accumulated, shared duplicated), got %d: %+v", len(entries), entries)
	}

	byKey := map[string]string{}
	for _, entry := range entries {
		byKey[entry.Name+"@"+entry.Level] = entry.Description
	}
	if byKey["proj-skill@project"] != "project skill" {
		t.Errorf("project skill missing, got %+v", byKey)
	}
	if byKey["user-skill@user"] != "user skill" {
		t.Errorf("user skill missing, got %+v", byKey)
	}
	if byKey["sys-skill@system"] != "system skill" {
		t.Errorf("system skill missing, got %+v", byKey)
	}
	// 同名 shared 在展示层并列两条：project 与 user。
	if byKey["shared@project"] != "project shared" || byKey["shared@user"] != "user shared" {
		t.Errorf("shared should appear in both project and user levels, got %+v", byKey)
	}
}

func TestRepositoryOverridesByLevelOrder(t *testing.T) {
	module, _ := newTestModule(t)

	summaries := module.Repository().Summaries()
	byName := map[string]string{}
	for _, summary := range summaries {
		byName[summary.Name] = summary.Description
	}
	// 同名 shared 运行时只剩 project 那条（项目级覆盖用户级）。
	if byName["shared"] != "project shared" {
		t.Errorf("runtime repository should keep project-level shared, got %q", byName["shared"])
	}
	for _, name := range []string{"proj-skill", "user-skill", "sys-skill"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("runtime repository missing skill %q", name)
		}
	}
}

func TestNewWithMissingDirectoriesDoesNotError(t *testing.T) {
	// 显式注入不存在的空目录，避免依赖真实用户目录状态。
	module, err := New(Dependencies{
		ProjectRoot:     t.TempDir(),
		UserSkillsDir:   filepath.Join(t.TempDir(), "nope"),
		SystemSkillsDir: filepath.Join(t.TempDir(), "nope"),
	})
	if err != nil {
		t.Fatalf("New with no skills directories should not error: %v", err)
	}
	if len(module.List()) != 0 {
		t.Errorf("expected empty list, got %+v", module.List())
	}
	if module.Repository() == nil {
		t.Error("repository should still be created")
	}
}
