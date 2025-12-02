package core

import (
	"context"
)

// AccessTokenProvider AccessToken 提供者接口
// 各产品（小程序、企业微信、公众号）需实现此接口
type AccessTokenProvider interface {
	// GetToken 获取 AccessToken
	// 实现应处理缓存和自动刷新逻辑
	//
	// 参数:
	//   - ctx: 上下文
	//
	// 返回:
	//   - string: 可用于调用微信 API 的 access_token
	//   - error: 可能的错误
	//
	// 错误:
	//   - 获取或刷新 token 失败
	//   - 配置缺失导致无法获取 token
	GetToken(ctx context.Context) (string, error)

	// RefreshToken 强制刷新 AccessToken
	// 用于 token 失效时主动刷新
	//
	// 参数:
	//   - ctx: 上下文
	//
	// 返回:
	//   - string: 新的 access_token
	//   - error: 可能的错误
	//
	// 错误:
	//   - 向微信服务端刷新 token 失败
	//   - 微信接口限频导致刷新失败
	RefreshToken(ctx context.Context) (string, error)
}
