package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
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

// WithLogger 设置日志记录器
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewClient 创建 HTTP 客户端
func NewClient(tokenProvider AccessTokenProvider, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		baseURL:       DefaultBaseURL,
		tokenProvider: tokenProvider,
		logger:        slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Get 发送 GET 请求
func (c *Client) Get(ctx context.Context, path string, query map[string]string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, query, nil)
}

// PostJSON 发送 POST JSON 请求
func (c *Client) PostJSON(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, nil, body)
}

// PostJSONWithQuery 发送带查询参数的 POST JSON 请求
func (c *Client) PostJSONWithQuery(ctx context.Context, path string, query map[string]string, body any) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, query, body)
}

// Upload 上传文件
func (c *Client) Upload(ctx context.Context, path string, fieldName string, fileName string, fileReader io.Reader, extraFields map[string]string) ([]byte, error) {
	// 获取 access_token
	token, err := c.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 构建 URL
	reqURL, err := c.buildURL(path, map[string]string{"access_token": token})
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	// 构建 multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件字段
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, fileReader); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}

	// 添加额外字段
	for key, value := range extraFields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write field %s: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c.logger.Debug("upload request",
		slog.String("method", http.MethodPost),
		slog.String("url", reqURL),
		slog.String("filename", fileName),
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

	c.logger.Debug("upload response",
		slog.Int("status", resp.StatusCode),
		slog.String("body", string(respBody)),
	)

	return respBody, nil
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string, query map[string]string, body any) ([]byte, error) {
	// 获取 access_token
	token, err := c.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 复制 query map 并添加 access_token，避免修改调用方的原始 map
	params := make(map[string]string, len(query)+1)
	for k, v := range query {
		params[k] = v
	}
	params["access_token"] = token

	// 构建 URL
	reqURL, err := c.buildURL(path, params)
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

// buildURL 构建完整 URL
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
