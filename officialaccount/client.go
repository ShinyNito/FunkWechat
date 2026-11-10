package officialaccount

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/ShinyNito/FunkWechat/v2/core"
)

const (
	accessTokenCacheKeyPrefix = "officialaccount:access_token:"
	tokenExpireBuffer         = 300
)

type Config struct {
	AppID      string
	AppSecret  string
	Cache      core.Cache
	HTTPClient *http.Client
	Logger     *slog.Logger
	BaseURL    string
}

type Client struct {
	cfg          Config
	apiClient    *core.Client
	tokenManager *core.TokenManager
	ticketMu     sync.Mutex
}

func New(cfg Config) (*Client, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	tokenClient, err := core.NewClient(core.ClientConfig{
		BaseURL:    cfg.BaseURL,
		HTTPClient: cfg.HTTPClient,
		Logger:     cfg.Logger,
	})
	if err != nil {
		return nil, err
	}

	tokenManager, err := core.NewTokenManager(core.TokenManagerConfig{
		Cache:               cfg.Cache,
		CacheKey:            accessTokenCacheKeyPrefix + cfg.AppID,
		ExpireBufferSeconds: tokenExpireBuffer,
		Logger:              cfg.Logger,
		Fetcher:             core.StableTokenFetcher(tokenClient, cfg.AppID, cfg.AppSecret),
	})
	if err != nil {
		return nil, err
	}

	apiClient, err := core.NewClient(core.ClientConfig{
		BaseURL:       cfg.BaseURL,
		HTTPClient:    cfg.HTTPClient,
		TokenProvider: tokenManager,
		Logger:        cfg.Logger,
	})
	if err != nil {
		return nil, err
	}

	return &Client{cfg: cfg, apiClient: apiClient, tokenManager: tokenManager}, nil
}

func (c *Client) Config() Config {
	return c.cfg
}

func (c *Client) AccessTokenProvider() core.AccessTokenProvider {
	return c.tokenManager
}

func normalizeConfig(cfg Config) Config {
	if cfg.Cache == nil {
		cfg.Cache = core.NewMemoryCache()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.AppID) == "" {
		return fmt.Errorf("appid is required")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return fmt.Errorf("appsecret is required")
	}
	return nil
}
