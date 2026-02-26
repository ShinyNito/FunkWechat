package miniprogram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ShinyNito/FunkWechat/core"
)

const (
	accessTokenPath           = "/cgi-bin/token"
	accessTokenCacheKeyPrefix = "miniprogram:access_token:"
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
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
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
		Fetcher: func(ctx context.Context) (core.TokenFetchResult, error) {
			resp, err := core.NewTypedRequest[accessTokenResponse](tokenClient).
				Path(accessTokenPath).
				Query("grant_type", "client_credential").
				Query("appid", cfg.AppID).
				Query("secret", cfg.AppSecret).
				WithoutToken().
				Get(ctx)
			if err != nil {
				return core.TokenFetchResult{}, fmt.Errorf("request access token: %w", err)
			}
			return core.TokenFetchResult{Token: resp.AccessToken, ExpiresIn: resp.ExpiresIn}, nil
		},
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
