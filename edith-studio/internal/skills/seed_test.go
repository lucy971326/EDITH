package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedSystemSkillsIntoMaterializesBuiltin(t *testing.T) {
	target := filepath.Join(t.TempDir(), ".system")

	if err := seedSystemSkillsInto(target); err != nil {
		t.Fatalf("seedSystemSkillsInto: %v", err)
	}

	// 内置 skill-creator 的技能文件应全部物化。
	expected := []string{
		"skill-creator/SKILL.md",
		"skill-creator/license.txt",
		"skill-creator/scripts/init_skill.py",
		"skill-creator/scripts/quick_validate.py",
		"skill-creator/assets/skill-creator.png",
		"skill-creator/assets/skill-creator-small.svg",
	}
	for _, rel := range expected {
		path := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing materialized file %s: %v", rel, err)
		}
	}

	// SKILL.md 应为 EDITH 适配版（无 Codex 引用、路径指向 ~/.edith/skills）。
	skillMD, err := os.ReadFile(filepath.Join(target, "skill-creator", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(skillMD)
	if strings.Contains(content, "Codex") {
		t.Errorf("SKILL.md still references Codex")
	}
	if !strings.Contains(content, "~/.edith/skills") {
		t.Errorf("SKILL.md should reference ~/.edith/skills for auto-discovery")
	}
}

func TestSeedSystemSkillsIntoOverwritesOldContent(t *testing.T) {
	target := filepath.Join(t.TempDir(), ".system")
	if err := os.MkdirAll(filepath.Join(target, "skill-creator"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(target, "skill-creator", "STALE.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := seedSystemSkillsInto(target); err != nil {
		t.Fatalf("seedSystemSkillsInto: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file should have been removed, stat err = %v", err)
	}
}
