// Package miniprogram 微信小程序服务端接口。
package miniprogram

import "github.com/ShinyNito/FunkWechat/v3/core"

type Client struct {
	*core.App
}

func New(cfg core.AppConfig) (*Client, error) {
	app, err := core.NewApp(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{App: app}, nil
}
