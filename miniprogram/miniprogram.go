package miniprogram

import (
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
func New(cfg *Config) *MiniProgram {
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
	}
	if cfg.HTTPClient != nil {
		clientOpts = append(clientOpts, core.WithHTTPClient(cfg.HTTPClient))
	}

	// 创建 HTTP 客户端
	client := core.NewClient(accessToken, clientOpts...)

	return &MiniProgram{
		config:      cfg,
		accessToken: accessToken,
		client:      client,
	}
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
