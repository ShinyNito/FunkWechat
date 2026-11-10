package core

import "context"

// AccessTokenProvider 为请求提供 access_token。
type AccessTokenProvider interface {
	// GetToken 返回可用的 access_token，优先读缓存。
	GetToken(ctx context.Context) (string, error)
	// RefreshToken 跳过缓存重新获取 access_token，用于 token 失效后的重试。
	RefreshToken(ctx context.Context) (string, error)
}
