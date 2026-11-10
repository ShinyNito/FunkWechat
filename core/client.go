package core

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.weixin.qq.com"
	DefaultTimeout = 30 * time.Second
	// maxResponseBytes 响应体上限。微信接口的 JSON 响应和小程序码图片都远小于此值。
	maxResponseBytes = 10 << 20
)

type ClientConfig struct {
	BaseURL string
	// HTTPClient 为空时使用带连接池的默认客户端。
	HTTPClient    *http.Client
	TokenProvider AccessTokenProvider
	Logger        *slog.Logger
}

// Client 发送微信 API 请求，负责附加 access_token、解析 errcode 和记录日志。
type Client struct {
	http    *http.Client
	baseURL string
	token   AccessTokenProvider
	logger  *slog.Logger
}

func NewClient(cfg ClientConfig) *Client {
	baseURL := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{http: httpClient, baseURL: baseURL, token: cfg.TokenProvider, logger: logger}
}

// newDefaultHTTPClient 所有请求都发往同一个微信域名，默认每个主机只保留 2 个空闲连接，
// 并发时会反复建立 TLS 连接，这里调大。
func newDefaultHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 32
	return &http.Client{Timeout: DefaultTimeout, Transport: transport}
}

// Request 创建一个请求。path 以 / 开头，例如 /cgi-bin/message/subscribe/send。
func (c *Client) Request(path string) *Request {
	return &Request{client: c, path: path}
}

func (c *Client) logAttempt(ctx context.Context, method, path string, reqBody []byte, resp response, elapsed time.Duration, err error) {
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	attrs := []slog.Attr{
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", resp.status),
		slog.Duration("elapsed", elapsed),
	}
	if len(reqBody) > 0 {
		attrs = append(attrs, slog.String("request", redactBody(reqBody)))
	}
	if len(resp.body) > 0 {
		attrs = append(attrs, slog.String("response", redactBody(resp.body)))
	}
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "wechat api call", attrs...)
}
