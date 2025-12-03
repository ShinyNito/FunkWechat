package miniprogram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ShinyNito/FunkWechat/core"
)

const (
	// accessTokenPath 获取 access_token 的路径
	accessTokenPath = "/cgi-bin/token"
	// 缓存 key 前缀
	accessTokenCacheKeyPrefix = "miniprogram:access_token:"
	// token 提前过期时间（秒），避免边界问题
	tokenExpireBuffer = 300
)

// accessTokenResponse access_token 响应
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// AccessToken 小程序 AccessToken 管理
type AccessToken struct {
	appID     string
	appSecret string
	cache     core.Cache
	client    *core.Client
	logger    *slog.Logger
	mu        sync.Mutex
}

// NewAccessToken 创建 AccessToken 实例
func NewAccessToken(appID, appSecret string, cache core.Cache, httpClient *http.Client, logger *slog.Logger) *AccessToken {
	if logger == nil {
		logger = slog.Default()
	}

	// 创建不需要 token 的 core.Client（传入 nil tokenProvider）
	clientOpts := []core.ClientOption{
		core.WithLogger(logger),
	}
	if httpClient != nil {
		clientOpts = append(clientOpts, core.WithHTTPClient(httpClient))
	}

	return &AccessToken{
		appID:     appID,
		appSecret: appSecret,
		cache:     cache,
		client:    core.NewClient(clientOpts...), // nil tokenProvider
		logger:    logger,
	}
}

// GetToken 获取 AccessToken（优先从缓存获取）
func (at *AccessToken) GetToken(ctx context.Context) (string, error) {
	cacheKey := at.cacheKey()

	// 尝试从缓存获取
	if token, ok := at.cache.Get(ctx, cacheKey); ok {
		at.logger.Debug("access_token from cache",
			slog.String("appid", at.appID),
		)
		return token, nil
	}

	// 缓存未命中，刷新 token
	return at.RefreshToken(ctx)
}

// RefreshToken 强制刷新 AccessToken
func (at *AccessToken) RefreshToken(ctx context.Context) (string, error) {
	at.mu.Lock()
	defer at.mu.Unlock()

	cacheKey := at.cacheKey()

	// 双重检查，避免并发刷新
	if token, ok := at.cache.Get(ctx, cacheKey); ok {
		return token, nil
	}

	at.logger.Info("refreshing access_token",
		slog.String("appid", at.appID),
	)

	// 使用 core.Client 请求微信 API
	body, err := at.client.Request().
		Path(accessTokenPath).
		Query("grant_type", "client_credential").
		Query("appid", at.appID).
		Query("secret", at.appSecret).
		WithoutToken(). // 不需要 access_token
		Get(ctx)
	if err != nil {
		return "", fmt.Errorf("request access_token: %w", err)
	}

	// 使用 Response 解析，自动处理微信错误
	resp := NewResponse[accessTokenResponse](body)
	result, err := resp.Decode()
	if err != nil {
		at.logger.Error("refresh access_token failed",
			slog.String("appid", at.appID),
			slog.Any("error", err),
		)
		return "", err
	}

	// 缓存 token（提前 5 分钟过期，最小缓存 1 秒防止出现负值）
	ttlSeconds := max(result.ExpiresIn-tokenExpireBuffer, 1)
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := at.cache.Set(ctx, cacheKey, result.AccessToken, ttl); err != nil {
		at.logger.Warn("cache access_token failed",
			slog.String("appid", at.appID),
			slog.Any("error", err),
		)
	}

	at.logger.Info("access_token refreshed",
		slog.String("appid", at.appID),
		slog.Int("expires_in", result.ExpiresIn),
	)

	return result.AccessToken, nil
}

// cacheKey 生成缓存 key
func (at *AccessToken) cacheKey() string {
	return accessTokenCacheKeyPrefix + at.appID
}

// 确保 AccessToken 实现了 AccessTokenProvider 接口
var _ core.AccessTokenProvider = (*AccessToken)(nil)
