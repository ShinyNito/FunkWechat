package officialaccount

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/ShinyNito/FunkWechat/core"
)

// Config 公众号配置
type Config struct {
	// AppID 公众号 AppID（必填）
	AppID string
	// AppSecret 公众号 AppSecret（必填）
	AppSecret string
	// Cache 缓存实现（可选，默认使用内存缓存）
	Cache core.Cache
	// HTTPClient 自定义 HTTP 客户端（可选）
	HTTPClient *http.Client
	// Logger 日志记录器（可选，默认使用 slog.Default()）
	Logger *slog.Logger
}

// OfficialAccount 公众号实例
type OfficialAccount struct {
	config      *Config
	accessToken *AccessToken
	client      *core.Client
	ticketMu    sync.Mutex // 防止并发刷新 ticket
}

// New 创建公众号实例
func New(cfg *Config) (*OfficialAccount, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid officialaccount config: %w", err)
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

	return &OfficialAccount{
		config:      cfg,
		accessToken: accessToken,
		client:      client,
	}, nil
}

// GetClient 获取 HTTP 客户端
func (oa *OfficialAccount) GetClient() *core.Client {
	return oa.client
}

// GetAccessToken 获取 AccessToken 管理器
func (oa *OfficialAccount) GetAccessToken() *AccessToken {
	return oa.accessToken
}

// GetConfig 获取配置
func (oa *OfficialAccount) GetConfig() *Config {
	return oa.config
}

// Get 发送 GET 请求并解析响应到 result（带 access_token）
func (oa *OfficialAccount) Get(ctx context.Context, path string, query map[string]string, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid get result: %w", err)
	}

	body, err := oa.client.Request().
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
func (oa *OfficialAccount) GetWithoutToken(ctx context.Context, path string, query map[string]string, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid get result: %w", err)
	}

	body, err := oa.client.Request().
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
func (oa *OfficialAccount) Post(ctx context.Context, path string, reqBody any, result any) error {
	if err := validateDecodeTarget(result); err != nil {
		return fmt.Errorf("invalid post result: %w", err)
	}

	body, err := oa.client.Request().
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

// GetRaw 获取原始响应
func (oa *OfficialAccount) GetRaw(ctx context.Context, path string, query map[string]string, withToken bool) ([]byte, error) {
	builder := oa.client.Request().Path(path).QueryMap(query)
	if !withToken {
		builder = builder.WithoutToken()
	}
	return builder.Get(ctx)
}

// PostRaw 发送 POST 请求获取原始响应
func (oa *OfficialAccount) PostRaw(ctx context.Context, path string, body any, withToken bool) ([]byte, error) {
	builder := oa.client.Request().Path(path).Body(body)
	if !withToken {
		builder = builder.WithoutToken()
	}
	return builder.Post(ctx)
}
