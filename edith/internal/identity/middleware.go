// Package identity 提供 Clerk 身份解析：把请求里的 Bearer Token 验签后，
// 把 clerk_user_id 注入请求 context。
package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

type ctxKey struct{}

// ErrNoUser 表示请求 context 里没有 clerk_user_id。
var ErrNoUser = errors.New("identity: no clerk_user_id in context")

// WithClerkUserID 把 clerk_user_id 注入 ctx（一般中间件用）。
func WithClerkUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// ClerkUserIDFromContext 从 ctx 读 clerk_user_id；没有返回 ErrNoUser。
func ClerkUserIDFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(ctxKey{}).(string)
	if !ok || v == "" {
		return "", ErrNoUser
	}
	return v, nil
}

// Middleware 包装 Clerk 官方中间件；验签成功后再把 clerk_user_id 写入 ctx。
// 使用 clerk.RequireHeaderAuthorization 提供的 ctx → claims → Subject。
func Middleware() func(http.Handler) http.Handler {
	requireAuth := clerkhttp.RequireHeaderAuthorization()
	injectUserID := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok || claims == nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := WithClerkUserID(r.Context(), claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return func(next http.Handler) http.Handler {
		return requireAuth(injectUserID(next))
	}
}