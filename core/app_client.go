package core

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

// AppConfig 使用 appid/appsecret 获取 access_token 的应用配置，小程序和公众号共用。
type AppConfig struct {
	AppID     string
	AppSecret string
	// Cache 存放 access_token 等凭证，为空时使用进程内存缓存。
	Cache      Cache
	HTTPClient *http.Client
	Logger     *slog.Logger
	BaseURL    string
}

// AppClient 应用的 API 客户端及其 access_token 管理器。
type AppClient struct {
	// Config 补全默认值后的配置。
	Config AppConfig
	API    *Client
	Token  *credential.Credential
}

// NewAppClient 校验配置，创建通过稳定版接口获取 access_token 的 API 客户端。
// tokenCacheKeyPrefix 用于区分不同产品的 token 缓存，实际缓存键为前缀加 appid。
func NewAppClient(cfg AppConfig, tokenCacheKeyPrefix string) (*AppClient, error) {
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

	tokenClient := NewClient(ClientConfig{
		BaseURL:    cfg.BaseURL,
		HTTPClient: cfg.HTTPClient,
		Logger:     cfg.Logger,
	})

	token := credential.New(cfg.Cache, tokenCacheKeyPrefix+cfg.AppID,
		StableTokenFetcher(tokenClient, cfg.AppID, cfg.AppSecret), cfg.Logger)

	api := NewClient(ClientConfig{
		BaseURL:    cfg.BaseURL,
		HTTPClient: cfg.HTTPClient,
		Token:      token,
		Logger:     cfg.Logger,
	})

	return &AppClient{Config: cfg, API: api, Token: token}, nil
}
