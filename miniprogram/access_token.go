package miniprogram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ShinyNito/FunkWechat/core"
)

const (
	// accessTokenURL 获取 access_token 的 URL
	accessTokenURL = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
	// 缓存 key 前缀
	accessTokenCacheKeyPrefix = "miniprogram:access_token:"
	// token 提前过期时间（秒），避免边界问题
	tokenExpireBuffer = 300
)

// accessTokenResponse access_token 响应
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// AccessToken 小程序 AccessToken 管理
type AccessToken struct {
	appID      string
	appSecret  string
	cache      core.Cache
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewAccessToken 创建 AccessToken 实例
func NewAccessToken(appID, appSecret string, cache core.Cache, httpClient *http.Client, logger *slog.Logger) *AccessToken {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &AccessToken{
		appID:      appID,
		appSecret:  appSecret,
		cache:      cache,
		httpClient: httpClient,
		logger:     logger,
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

	// 请求微信 API
	url := fmt.Sprintf(accessTokenURL, at.appID, at.appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := at.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result accessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	// 检查错误
	if result.ErrCode != 0 {
		at.logger.Error("refresh access_token failed",
			slog.String("appid", at.appID),
			slog.Int("errcode", result.ErrCode),
			slog.String("errmsg", result.ErrMsg),
		)
		return "", core.NewWechatError(result.ErrCode, result.ErrMsg)
	}

	// 缓存 token（提前 5 分钟过期）
	ttl := time.Duration(result.ExpiresIn-tokenExpireBuffer) * time.Second
	if ttl < 0 {
		ttl = time.Duration(result.ExpiresIn) * time.Second
	}

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
