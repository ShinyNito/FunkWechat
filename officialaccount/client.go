package officialaccount

import (
	"github.com/ShinyNito/FunkWechat/v3/core"
	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

const accessTokenCacheKeyPrefix = "officialaccount:access_token:"

// Config 公众号配置。
type Config = core.AppConfig

type Client struct {
	cfg       Config
	apiClient *core.Client
	token     *credential.Credential
	tickets   map[TicketType]*credential.Credential
}

func New(cfg Config) (*Client, error) {
	app, err := core.NewAppClient(cfg, accessTokenCacheKeyPrefix)
	if err != nil {
		return nil, err
	}
	c := &Client{cfg: app.Config, apiClient: app.API, token: app.Token}
	c.tickets = newTicketCredentials(c)
	return c, nil
}

// Request 创建请求，用于调用未封装的接口。
func (c *Client) Request(path string) *core.Request {
	return c.apiClient.Request(path)
}

func (c *Client) Config() Config {
	return c.cfg
}

// AccessToken 返回 access_token 凭证。
func (c *Client) AccessToken() *credential.Credential {
	return c.token
}
