package core

import (
	"context"
	"fmt"
	"time"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

// StableTokenPath 稳定版 access_token 接口路径。
const StableTokenPath = "/cgi-bin/stable_token"

type stableTokenRequest struct {
	GrantType    string `json:"grant_type"`
	AppID        string `json:"appid"`
	Secret       string `json:"secret"`
	ForceRefresh bool   `json:"force_refresh"`
}

type stableTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// StableTokenFetcher 返回通过稳定版接口获取 access_token 的 credential.FetchFunc。
//
// 普通模式（force_refresh=false）下，有效期内重复获取返回同一个 token，
// 也不会让其他服务通过 /cgi-bin/token 或本接口拿到的 token 失效，
// 因此多个进程、多个服务可以各自获取而互不影响。
// 强制刷新每天限 20 次且会作废旧 token，这里不使用。
func StableTokenFetcher(client *Client, appID, appSecret string) credential.FetchFunc {
	return func(ctx context.Context) (string, time.Duration, error) {
		resp, err := client.Request(StableTokenPath).
			Body(stableTokenRequest{
				GrantType: "client_credential",
				AppID:     appID,
				Secret:    appSecret,
			}).
			WithoutToken().
			Post[stableTokenResponse](ctx)
		if err != nil {
			return "", 0, fmt.Errorf("request stable access token: %w", err)
		}
		return resp.AccessToken, time.Duration(resp.ExpiresIn) * time.Second, nil
	}
}
