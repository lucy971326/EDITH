package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v69/github"
)

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var (
	appID      string
	privateKey *rsa.PrivateKey
	cache      = make(map[int64]*cachedToken)
	mu         sync.RWMutex
)

// Init 加载私钥，启动时调用一次
func Init(id, keyPath string) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		log.Fatalf("读取私钥失败: %v", err)
	}

	block, _ := pem.Decode(data)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("解析私钥失败: %v", err)
	}

	appID = id
	privateKey = key
}

// GetInstallationToken 获取指定安装的访问令牌（带缓存）
func GetInstallationToken(ctx context.Context, installationID int64) (string, error) {
	mu.RLock()
	if c, ok := cache[installationID]; ok && time.Now().Before(c.expiresAt) {
		mu.RUnlock()
		return c.token, nil
	}
	mu.RUnlock()

	jwtStr, err := generateJWT()
	if err != nil {
		return "", err
	}

	client := github.NewClient(nil).WithAuthToken(jwtStr)
	token, _, err := client.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", err
	}

	mu.Lock()
	cache[installationID] = &cachedToken{
		token:     token.GetToken(),
		expiresAt: token.GetExpiresAt().Add(-5 * time.Minute),
	}
	mu.Unlock()

	return token.GetToken(), nil
}

func generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": appID,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return t.SignedString(privateKey)
}
