package miniprogram

import "github.com/ShinyNito/FunkWechat/v2/core"

const accessTokenCacheKeyPrefix = "miniprogram:access_token:"

// Config 小程序配置。
type Config = core.AppConfig

type Client struct {
	cfg          Config
	apiClient    *core.Client
	tokenManager *core.TokenManager
}

func New(cfg Config) (*Client, error) {
	app, err := core.NewAppClient(cfg, accessTokenCacheKeyPrefix)
	if err != nil {
		return nil, err
	}
	return &Client{cfg: app.Config, apiClient: app.API, tokenManager: app.Tokens}, nil
}

func (c *Client) Config() Config {
	return c.cfg
}

func (c *Client) AccessTokenProvider() core.AccessTokenProvider {
	return c.tokenManager
}
