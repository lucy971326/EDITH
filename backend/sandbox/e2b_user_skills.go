package sandbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/eric642/e2b-go-sdk/volume"
)

// userSkillVolume 是一个用户永久 Skill 空间在 E2B 中的定位信息。
// Token 只留在后端，用于直接读写 Volume 内容。
type userSkillVolume struct {
	ID    string
	Name  string
	Token string
}

func initUserSkillVolumesTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_skill_volumes (
		user_id      TEXT PRIMARY KEY,
		volume_id    TEXT NOT NULL,
		volume_name  TEXT NOT NULL,
		volume_token TEXT NOT NULL,
		created_at   INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create user_skill_volumes table: %w", err)
	}
	return nil
}

// LoadUserSkillsOverview 直接从用户 Volume 根目录读取 OVERVIEW.md。
// 这是用户级数据，不需要创建或连接 Sandbox。
func (p *E2BProvider) LoadUserSkillsOverview(ctx context.Context, userID string) (string, error) {
	userVolume, err := p.ensureUserSkillVolume(ctx, userID)
	if err != nil {
		return "", err
	}

	connected, err := volume.Connect(ctx, userVolume.ID, userVolume.Token, volume.Options{
		Config: p.client.Config(),
	})
	if err != nil {
		return "", fmt.Errorf("connect user skill volume: %w", err)
	}

	content, err := connected.ReadFile(ctx, "OVERVIEW.md")
	if err != nil {
		return "", fmt.Errorf("read user skills overview: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

// ensureUserSkillVolume returns the user's existing Volume, or creates it once.
func (p *E2BProvider) ensureUserSkillVolume(ctx context.Context, userID string) (userSkillVolume, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return userSkillVolume{}, fmt.Errorf("user ID is required")
	}

	stored, err := p.findUserSkillVolume(ctx, userID)
	if err == nil {
		return stored, nil
	}
	if err != sql.ErrNoRows {
		return userSkillVolume{}, err
	}

	// 首次创建很少发生；串行保证同一用户不会创建两个 Volume。
	p.userSkillsMu.Lock()
	defer p.userSkillsMu.Unlock()

	stored, err = p.findUserSkillVolume(ctx, userID)
	if err == nil {
		return stored, nil
	}
	if err != sql.ErrNoRows {
		return userSkillVolume{}, err
	}

	created, err := volume.Create(ctx, userSkillVolumeName(userID), volume.Options{
		Config: p.client.Config(),
	})
	if err != nil {
		return userSkillVolume{}, fmt.Errorf("create user skill volume: %w", err)
	}

	// 初始化空索引：后续读取无需处理“文件不存在”。
	if err := created.WriteFile(ctx, "OVERVIEW.md", []byte{}); err != nil {
		_ = created.Delete(ctx)
		return userSkillVolume{}, fmt.Errorf("initialize user skills overview: %w", err)
	}

	stored = userSkillVolume{
		ID:    created.ID,
		Name:  created.Name,
		Token: created.Token(),
	}
	if err := p.saveUserSkillVolume(ctx, userID, stored); err != nil {
		_ = created.Delete(ctx)
		return userSkillVolume{}, err
	}
	return stored, nil
}

func (p *E2BProvider) findUserSkillVolume(ctx context.Context, userID string) (userSkillVolume, error) {
	var stored userSkillVolume
	err := p.db.QueryRowContext(ctx, `
		SELECT volume_id, volume_name, volume_token
		FROM user_skill_volumes
		WHERE user_id = ?
	`, userID).Scan(&stored.ID, &stored.Name, &stored.Token)
	if err != nil {
		return userSkillVolume{}, err
	}
	return stored, nil
}

func (p *E2BProvider) saveUserSkillVolume(ctx context.Context, userID string, stored userSkillVolume) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO user_skill_volumes (
			user_id, volume_id, volume_name, volume_token, created_at
		) VALUES (?, ?, ?, ?, ?)
	`, userID, stored.ID, stored.Name, stored.Token, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("save user skill volume: %w", err)
	}
	return nil
}

func userSkillVolumeName(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return "edith-skills-" + hex.EncodeToString(sum[:16])
}
