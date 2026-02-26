package miniprogram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ShinyNito/FunkWechat/core"
)

// Config 小程序配置
type Config struct {
	// AppID 小程序 AppID（必填）
	AppID string
	// AppSecret 小程序 AppSecret（必填）
	AppSecret string
	// Cache 缓存实现（可选，默认使用内存缓存）
	Cache core.Cache
	// HTTPClient 自定义 HTTP 客户端（可选）
	HTTPClient *http.Client
	// Logger 日志记录器（可选，默认使用 slog.Default()）
	Logger *slog.Logger
}

// MiniProgram 小程序实例
type MiniProgram struct {
	config      *Config
	accessToken *AccessToken
	client      *core.Client
}

// New 创建小程序实例
func New(cfg *Config) (*MiniProgram, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid miniprogram config: %w", err)
	}

	// 默认缓存
	if cfg.Cache == nil {
		cfg.Cache = core.NewMemoryCache()
	}

	// 默认日志
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// 创建 AccessToken 管理器
	accessToken := NewAccessToken(
		cfg.AppID,
		cfg.AppSecret,
		cfg.Cache,
		cfg.HTTPClient,
		cfg.Logger,
	)

	// 创建 HTTP 客户端选项
	clientOpts := []core.ClientOption{
		core.WithLogger(cfg.Logger),
		core.WithTokenProvider(accessToken),
	}
	if cfg.HTTPClient != nil {
		clientOpts = append(clientOpts, core.WithHTTPClient(cfg.HTTPClient))
	}

	// 创建 HTTP 客户端
	client := core.NewClient(clientOpts...)

	return &MiniProgram{
		config:      cfg,
		accessToken: accessToken,
		client:      client,
	}, nil
}

// GetClient 获取 HTTP 客户端
// 用于调用任意微信 API
func (mp *MiniProgram) GetClient() *core.Client {
	return mp.client
}

// GetAccessToken 获取 AccessToken 管理器
// 用于手动管理 token（如强制刷新）
func (mp *MiniProgram) GetAccessToken() *AccessToken {
	return mp.accessToken
}

// GetConfig 获取配置
func (mp *MiniProgram) GetConfig() *Config {
	return mp.config
}

// Get 发送 GET 请求并解析响应到 result（带 access_token）
// 用于调用 SDK 尚未封装的新 API
//
// 参数:
//   - ctx: 上下文
//   - path: 请求路径，如 "/cgi-bin/user/info"
//   - query: 查询参数
//   - result: 结果指针，响应将解析到此变量
//
// 示例:
//
//	var userInfo UserInfo
//	err := mp.Get(ctx, "/cgi-bin/user/info", map[string]string{
//	    "openid": "xxx",
//	}, &userInfo)
func (mp *MiniProgram) Get(ctx context.Context, path string, query map[string]string, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid get result: %w", err)
	}

	body, err := mp.client.Request().
		Path(path).
		QueryMap(query).
		Get(ctx)
	if err != nil {
		return err
	}

	resp := NewResponse[any](body)
	if err := resp.Error(); err != nil {
		return err
	}

	return resp.DecodeInto(result)
}

// GetWithoutToken 发送 GET 请求（不带 access_token）并解析响应到 result
func (mp *MiniProgram) GetWithoutToken(ctx context.Context, path string, query map[string]string, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid get result: %w", err)
	}

	body, err := mp.client.Request().
		Path(path).
		QueryMap(query).
		WithoutToken().
		Get(ctx)
	if err != nil {
		return err
	}

	resp := NewResponse[any](body)
	if err := resp.Error(); err != nil {
		return err
	}

	return resp.DecodeInto(result)
}

// Post 发送 POST 请求并解析响应到 result（带 access_token）
// 用于调用 SDK 尚未封装的新 API
//
// 参数:
//   - ctx: 上下文
//   - path: 请求路径，如 "/cgi-bin/message/custom/send"
//   - reqBody: 请求体
//   - result: 结果指针，响应将解析到此变量
//
// 示例:
//
//	var result SendMessageResult
//	err := mp.Post(ctx, "/cgi-bin/message/send", map[string]any{
//	    "touser":  "openid",
//	    "msgtype": "text",
//	    "text": map[string]string{"content": "Hello"},
//	}, &result)
func (mp *MiniProgram) Post(ctx context.Context, path string, reqBody any, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid post result: %w", err)
	}

	body, err := mp.client.Request().
		Path(path).
		Body(reqBody).
		Post(ctx)
	if err != nil {
		return err
	}

	resp := NewResponse[any](body)
	if err := resp.Error(); err != nil {
		return err
	}

	return resp.DecodeInto(result)
}

// PostWithQuery 发送带查询参数的 POST 请求（带 access_token）并解析响应到 result
func (mp *MiniProgram) PostWithQuery(ctx context.Context, path string, query map[string]string, reqBody any, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid post result: %w", err)
	}

	body, err := mp.client.Request().
		Path(path).
		QueryMap(query).
		Body(reqBody).
		Post(ctx)
	if err != nil {
		return err
	}

	resp := NewResponse[any](body)
	if err = resp.Error(); err != nil {
		return err
	}

	return resp.DecodeInto(result)
}

// GetRaw 获取原始响应（高级用法，用于特殊场景）
// 返回原始字节数据，由调用方自行处理
func (mp *MiniProgram) GetRaw(ctx context.Context, path string, query map[string]string, withToken bool) ([]byte, error) {
	builder := mp.client.Request().Path(path).QueryMap(query)
	if !withToken {
		builder = builder.WithoutToken()
	}
	return builder.Get(ctx)
}

// PostRaw 发送 POST 请求获取原始响应（高级用法）
func (mp *MiniProgram) PostRaw(ctx context.Context, path string, body any, withToken bool) ([]byte, error) {
	builder := mp.client.Request().Path(path).Body(body)
	if !withToken {
		builder = builder.WithoutToken()
	}
	return builder.Post(ctx)
}
