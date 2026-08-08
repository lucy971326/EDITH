package skills

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed builtin
var builtinFS embed.FS

// SeedSystemSkills 把二进制内嵌的内置技能全量物化到 ~/.edith/.system-skills/。
// 系统级技能是系统托管的命名空间：每次启动先清空再全量复制，保证与内置版本一致，
// 覆盖用户对系统目录的任何改动（系统托管，用户不应改这里）。不依赖 marker 版本比对，
// 全量替换最简单且升级自动带新技能。
func SeedSystemSkills() error {
	target := systemSkillsDir()
	if target == "" {
		return errors.New("cannot locate system skills directory")
	}
	return seedSystemSkillsInto(target)
}

// seedSystemSkillsInto 把 builtinFS 全量物化到 target（先清空再复制）。
// 独立成函数以便测试注入临时目录，不污染真实用户目录。
func seedSystemSkillsInto(target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(builtinFS, "builtin", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("builtin", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		contents, err := builtinFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, contents, 0o644)
	})
}
