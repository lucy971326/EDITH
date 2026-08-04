package volume

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
// overview.md 是可选的派生索引：用户尚未创建 Volume、文件不存在或 E2B
// 暂时不可用时都返回空字符串，让 Agent 继续运行；本地数据库错误仍会返回。
// 该方法不会创建远端 Volume。
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

	token, err := s.loadCurrentToken(ctx, value.ID)
	if err != nil {
		if isNotFoundVolumeError(err) {
			return "", nil
		}
		return s.skipUnavailableOverview(userID, value.ID, "load current token", err), nil
	}
	if token != value.Token {
		if err := s.store.updateToken(ctx, value.UserID, token); err != nil {
			return "", err
		}
	}
	if traceEnabled() {
		log.Printf("e2b volume overview user_id=%s volume_id=%s token_sha256=%x token_len=%d", userFingerprint(userID), value.ID, sha256.Sum256([]byte(token)), len(token))
	}

	opened, err := e2bvolume.Connect(ctx, value.ID, token, e2bvolume.Options{Config: s.config})
	if err != nil {
		if isNotFoundVolumeError(err) {
			return "", nil
		}
		return s.skipUnavailableOverview(userID, value.ID, "connect volume", err), nil
	}
	data, err := opened.ReadFile(ctx, UserOverviewPath)
	if err != nil {
		if isNotFoundVolumeError(err) {
			return "", nil
		}
		return s.skipUnavailableOverview(userID, value.ID, "read overview", err), nil
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Service) skipUnavailableOverview(userID, volumeID, action string, err error) string {
	log.Printf("skip unavailable user skill overview user_id=%s volume_id=%s action=%q error=%v", userFingerprint(userID), volumeID, action, err)
	return ""
}

func userFingerprint(userID string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(userID)))[:12]
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
	token, err := s.loadCurrentToken(ctx, value.ID)
	if err != nil {
		return nil, err
	}
	if token != value.Token {
		if err := s.store.updateToken(ctx, value.UserID, token); err != nil {
			return nil, err
		}
	}
	opened, err := e2bvolume.Connect(ctx, value.ID, token, e2bvolume.Options{Config: s.config})
	if err != nil {
		return nil, fmt.Errorf("connect user volume: %w", err)
	}
	return opened, nil
}

// loadCurrentToken 从 E2B 控制面读取当前 Volume Token。
// SDK 的 Connect 不会使用控制面响应里的 Token，因此由 Volume 模块统一取得并持久化。
func (s *Service) loadCurrentToken(ctx context.Context, volumeID string) (string, error) {
	endpoint := strings.TrimRight(s.config.APIURL, "/") + "/volumes/" + url.PathEscape(volumeID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build volume token request: %w", err)
	}
	if s.config.APIKey != "" {
		request.Header.Set("X-API-Key", s.config.APIKey)
	}
	if s.config.AccessToken != "" {
		request.Header.Set("Authorization", "Bearer "+s.config.AccessToken)
	}
	for key, value := range s.config.Headers {
		if request.Header.Get(key) == "" {
			request.Header.Set(key, value)
		}
	}

	response, err := s.config.ResolvedHTTPClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("load current volume token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return "", &e2b.VolumeError{Message: fmt.Sprintf("http %d: load current volume token", response.StatusCode)}
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode current volume token: %w", err)
	}
	if strings.TrimSpace(result.Token) == "" {
		return "", errors.New("load current volume token: E2B returned an empty token")
	}
	return strings.TrimSpace(result.Token), nil
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
