package core

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

const (
	defaultBaseURL = "https://api.weixin.qq.com"
	defaultTimeout = 30 * time.Second
	// maxResponseBytes 响应体上限。微信接口的 JSON 响应和小程序码图片都远小于此值。
	maxResponseBytes = 10 << 20
)

// client 发送微信 API 请求，负责附加 access_token、解析 errcode 和记录日志。
type client struct {
	http    *http.Client
	baseURL string
	// token 为空时请求不附加 access_token。
	token  *credential.Credential
	logger *slog.Logger
}

func newClient(baseURL string, httpClient *http.Client, logger *slog.Logger) *client {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &client{http: httpClient, baseURL: baseURL, logger: logger}
}

// newDefaultHTTPClient 所有请求都发往同一个微信域名，默认每个主机只保留 2 个空闲连接，
// 并发时会反复建立 TLS 连接，这里调大。
func newDefaultHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 32
	return &http.Client{Timeout: defaultTimeout, Transport: transport}
}

func (c *client) request(path string) *Request {
	return &Request{client: c, path: path}
}

func (c *client) logAttempt(ctx context.Context, method, path string, reqBody []byte, resp response, elapsed time.Duration, err error) {
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
