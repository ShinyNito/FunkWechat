package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.weixin.qq.com"
	DefaultTimeout = 30 * time.Second
)

type ClientConfig struct {
	BaseURL       string
	HTTPClient    *http.Client
	TokenProvider AccessTokenProvider
	Logger        *slog.Logger
}

type Client struct {
	httpClient    *http.Client
	baseURL       *url.URL
	tokenProvider AccessTokenProvider
	logger        *slog.Logger
}

func NewClient(cfg ClientConfig) (*Client, error) {
	baseURL := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		httpClient:    httpClient,
		baseURL:       parsedBaseURL,
		tokenProvider: cfg.TokenProvider,
		logger:        logger,
	}, nil
}

func (c *Client) Logger() *slog.Logger {
	return c.logger
}

func (c *Client) Request() *RequestBuilder {
	return newRequestBuilder(c)
}

func (c *Client) buildURL(path string, query map[string]string) (string, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}

	u := c.baseURL.ResolveReference(ref)
	if len(query) > 0 {
		values := u.Query()
		for key, value := range query {
			values.Set(key, value)
		}
		u.RawQuery = values.Encode()
	}

	return u.String(), nil
}

func (c *Client) logRequest(ctx context.Context, method, rawURL string, body []byte) {
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs := []slog.Attr{
		slog.String("method", method),
		slog.String("url", RedactURLQuery(rawURL)),
	}
	if len(body) > 0 {
		attrs = append(attrs, slog.String("body", string(body)))
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "http request", attrs...)
}

func (c *Client) logResponse(ctx context.Context, statusCode int, body []byte) {
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs := []slog.Attr{slog.Int("status", statusCode)}
	if len(body) > 0 {
		attrs = append(attrs, slog.String("body", string(body)))
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "http response", attrs...)
}
