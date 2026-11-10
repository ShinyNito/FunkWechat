// Package officialaccount 微信公众号服务端接口。
package officialaccount

import (
	"github.com/ShinyNito/FunkWechat/v3/core"
	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

type Client struct {
	*core.App
	tickets map[TicketType]*credential.Credential
}

func New(cfg core.AppConfig) (*Client, error) {
	app, err := core.NewApp(cfg)
	if err != nil {
		return nil, err
	}
	c := &Client{App: app}
	c.tickets = newTicketCredentials(c)
	return c, nil
}
