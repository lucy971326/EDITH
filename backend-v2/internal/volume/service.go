package volume

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/eric642/e2b-go-sdk"
	e2bvolume "github.com/eric642/e2b-go-sdk/volume"
	"github.com/google/uuid"
)

// Service 负责把 Clerk 用户绑定到唯一的 E2B Volume。
// 它是 Sandbox 和 Skills 模块使用的内部能力，不注册 HTTP 路由。
type Service struct {
	store  *store
	config e2b.Config
	mu     sync.Mutex
}

// ReadUserOverview 读取已有用户 Volume 中的 Skills 摘要。
// 用户尚未创建 Volume 或 overview.md 不存在时返回空字符串；不会创建远端 Volume。
func (s *Service) ReadUserOverview(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("volume requires a user ID")
	}

	value, err := s.store.load(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load user volume: %w", err)
	}

	opened, err := e2bvolume.Connect(ctx, value.ID, value.Token, e2bvolume.Options{Config: s.config})
	if err != nil {
		if isNotFoundVolumeError(err) {
			return "", nil
		}
		return "", fmt.Errorf("connect user volume: %w", err)
	}
	data, err := opened.ReadFile(ctx, UserOverviewPath)
	if err != nil {
		if isNotFoundVolumeError(err) {
			return "", nil
		}
		return "", fmt.Errorf("read user skill overview: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// MountForUser 返回当前用户的 Skills Volume 挂载信息；首次调用会创建远端 Volume。
func (s *Service) MountForUser(ctx context.Context, userID string) (Mount, error) {
	value, err := s.getOrCreateUserVolume(ctx, userID)
	if err != nil {
		return Mount{}, err
	}
	return Mount{Name: value.Name, Path: CustomSkillsPath}, nil
}

// OpenForUser 连接当前用户的 Volume，供需要读写完整用户文件的内部能力使用。
func (s *Service) OpenForUser(ctx context.Context, userID string) (*e2bvolume.Volume, error) {
	value, err := s.getOrCreateUserVolume(ctx, userID)
	if err != nil {
		return nil, err
	}
	opened, err := e2bvolume.Connect(ctx, value.ID, value.Token, e2bvolume.Options{Config: s.config})
	if err != nil {
		return nil, fmt.Errorf("connect user volume: %w", err)
	}
	return opened, nil
}

func (s *Service) getOrCreateUserVolume(ctx context.Context, userID string) (record, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return record{}, errors.New("volume requires a user ID")
	}

	// 单实例下串行保护首次创建，避免远端产生两个同用户 Volume。
	s.mu.Lock()
	defer s.mu.Unlock()

	value, err := s.store.load(ctx, userID)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return record{}, fmt.Errorf("load user volume: %w", err)
	}

	name := "edith-user-volume-" + uuid.NewString()
	created, err := e2bvolume.Create(ctx, name, e2bvolume.Options{Config: s.config})
	if err != nil {
		return record{}, fmt.Errorf("create user volume: %w", err)
	}
	value = record{UserID: userID, ID: created.ID, Name: created.Name, Token: created.Token()}
	if value.ID == "" || value.Name == "" || value.Token == "" {
		_ = created.Delete(ctx)
		return record{}, errors.New("create user volume returned incomplete information")
	}
	if err := s.store.save(ctx, value); err != nil {
		_ = created.Delete(ctx)
		return record{}, err
	}
	return value, nil
}

func isNotFoundVolumeError(err error) bool {
	var volumeErr *e2b.VolumeError
	if !errors.As(err, &volumeErr) {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(volumeErr.Message), "http 404:")
}
