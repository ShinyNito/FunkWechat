package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

const accessTokenCacheKeyPrefix = "funkwechat:access_token:"

// AppConfig 使用 appid/appsecret 鉴权的应用配置，小程序和公众号共用。
type AppConfig struct {
	AppID     string
	AppSecret string
	// Cache 存放 access_token 等凭证，为空时使用进程内存缓存。
	// 多个进程共用一个 Redis 时，可以实现 Cache 接口共享凭证。
	Cache Cache
	// HTTPClient 为空时使用带连接池、30 秒超时的默认客户端。
	HTTPClient *http.Client
	// Logger 为空时使用 slog.Default()。SDK 只输出 debug 级别的调用日志和少量 Warn。
	Logger *slog.Logger
	// BaseURL 为空时使用 https://api.weixin.qq.com，测试时可指向本地服务。
	BaseURL string
}

// App 一个小程序或公众号应用：负责 access_token 的获取、缓存和失效重试。
// 产品包嵌入 *App，只实现各自的业务接口。
type App struct {
	config AppConfig
	client *client
}

// NewApp 校验配置并创建应用。access_token 通过稳定版接口获取，与其他服务并存时不会互相作废。
func NewApp(cfg AppConfig) (*App, error) {
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, fmt.Errorf("appid is required")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, fmt.Errorf("appsecret is required")
	}
	if cfg.Cache == nil {
		cfg.Cache = NewMemoryCache()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	logger := cfg.Logger.With(slog.String("appid", cfg.AppID))
	c := newClient(cfg.BaseURL, cfg.HTTPClient, logger)
	c.token = credential.New(cfg.Cache, accessTokenCacheKeyPrefix+cfg.AppID,
		stableTokenFetcher(c, cfg.AppID, cfg.AppSecret), logger)
	return &App{config: cfg, client: c}, nil
}

// Config 返回补全默认值后的配置。
func (a *App) Config() AppConfig {
	return a.config
}

// Logger 返回带 appid 属性的 logger。
func (a *App) Logger() *slog.Logger {
	return a.client.logger
}

// Request 创建一个请求，path 以 / 开头，例如 /wxa/generate_urllink。
// 默认附加 access_token，用于调用 SDK 未封装的接口。
func (a *App) Request(path string) *Request {
	return a.client.request(path)
}

// AccessToken 返回当前的 access_token，优先读缓存。
func (a *App) AccessToken(ctx context.Context) (string, error) {
	return a.client.token.Get(ctx)
}
