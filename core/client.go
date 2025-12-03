package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL 微信 API 默认基础 URL
	DefaultBaseURL = "https://api.weixin.qq.com"
	// DefaultTimeout 默认超时时间
	DefaultTimeout = 30 * time.Second
)

// Client HTTP 客户端
type Client struct {
	httpClient    *http.Client
	baseURL       string
	tokenProvider AccessTokenProvider
	logger        *slog.Logger
}

// ClientOption 客户端选项
type ClientOption func(*Client)

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithBaseURL 设置基础 URL
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithTokenProvider 设置 AccessToken 提供器
func WithTokenProvider(provider AccessTokenProvider) ClientOption {
	return func(c *Client) {
		c.tokenProvider = provider
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewClient 创建 HTTP 客户端
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		baseURL: DefaultBaseURL,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Request 创建请求构建器（唯一的对外 API）
//
// 示例:
//
//	// GET 请求
//	body, err := client.Request().
//	    Path("/cgi-bin/user/info").
//	    Query("openid", "xxx").
//	    Get(ctx)
//
//	// POST 请求
//	body, err := client.Request().
//	    Path("/cgi-bin/message/send").
//	    Body(payload).
//	    Post(ctx)
//
//	// 不带 access_token
//	body, err := client.Request().
//	    Path("/sns/jscode2session").
//	    QueryMap(params).
//	    WithoutToken().
//	    Get(ctx)
func (c *Client) Request() *RequestBuilder {
	return newRequestBuilder(c)
}

// buildParams 构建参数（包内方法）
func (c *Client) buildParams(ctx context.Context, query map[string]string, shouldAddAccessToken bool) (map[string]string, error) {
	params := make(map[string]string, len(query)+1)
	maps.Copy(params, query)
	if shouldAddAccessToken {
		// 如果没有 tokenProvider，跳过添加 access_token
		if c.tokenProvider == nil {
			return params, nil
		}
		token, err := c.tokenProvider.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}
		params["access_token"] = token
	}
	return params, nil
}

// doRequest 执行 HTTP 请求（包内方法）
func (c *Client) doRequest(ctx context.Context, method, path string, query map[string]string, body any) ([]byte, error) {
	// 构建 URL
	reqURL, err := c.buildURL(path, query)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	// 构建请求体
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)

		c.logger.Debug("request body",
			slog.String("body", string(jsonBody)),
		)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("http request",
		slog.String("method", method),
		slog.String("url", reqURL),
	)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	c.logger.Debug("http response",
		slog.Int("status", resp.StatusCode),
		slog.String("body", string(respBody)),
	)

	return respBody, nil
}

// buildURL 构建完整 URL（包内方法）
func (c *Client) buildURL(path string, query map[string]string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}

	// 使用 ResolveReference 正确处理路径拼接
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}
	u := base.ResolveReference(ref)

	if len(query) > 0 {
		q := u.Query()
		for key, value := range query {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}
