package miniprogram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ShinyNito/FunkWechat/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCache struct {
	data map[string]string
}

func newStubCache() *stubCache {
	return &stubCache{data: make(map[string]string)}
}

func (c *stubCache) Get(ctx context.Context, key string) (string, bool) {
	v, ok := c.data[key]
	return v, ok
}

func (c *stubCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.data[key] = value
	return nil
}

func (c *stubCache) Delete(ctx context.Context, key string) error {
	delete(c.data, key)
	return nil
}

type rewriteTransport struct {
	target *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := *req
	newURL := *t.target
	newURL.Path = req.URL.Path
	newURL.RawQuery = req.URL.RawQuery
	newReq.URL = &newURL
	newReq.Host = t.target.Host
	newReq.RequestURI = ""
	return http.DefaultTransport.RoundTrip(&newReq)
}

func TestAccessToken_GetTokenAndRefresh(t *testing.T) {
	tests := []struct {
		name             string
		cacheValue       string
		serverResponse   accessTokenResponse
		wantToken        string
		wantErrCode      int
		expectServerHits int
	}{
		{
			name:             "cache hit returns without request",
			cacheValue:       "cached_token",
			expectServerHits: 0,
			wantToken:        "cached_token",
		},
		{
			name: "refresh success caches token",
			serverResponse: accessTokenResponse{
				AccessToken: "fresh_token",
				ExpiresIn:   7200,
				ErrCode:     0,
			},
			wantToken:        "fresh_token",
			expectServerHits: 1,
		},
		{
			name: "wechat error response",
			serverResponse: accessTokenResponse{
				ErrCode: core.ErrCodeInvalidToken,
				ErrMsg:  "invalid token",
			},
			wantErrCode:      core.ErrCodeInvalidToken,
			expectServerHits: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				respBytes, _ := json.Marshal(tt.serverResponse)
				_, _ = w.Write(respBytes)
			}))
			defer server.Close()

			targetURL, err := url.Parse(server.URL)
			require.NoError(t, err)

			cache := newStubCache()
			if tt.cacheValue != "" {
				cache.Set(context.Background(), accessTokenCacheKeyPrefix+"appid", tt.cacheValue, time.Hour)
			}

			at := NewAccessToken("appid", "secret", cache, &http.Client{
				Transport: &rewriteTransport{target: targetURL},
			}, nil)

			token, err := at.GetToken(context.Background())
			assert.Equal(t, tt.expectServerHits, hits)

			if tt.wantErrCode != 0 {
				require.Error(t, err)
				var we *core.WechatError
				assert.ErrorAs(t, err, &we)
				assert.Equal(t, tt.wantErrCode, we.ErrCode)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)

			// ensure cached after refresh
			if tt.cacheValue == "" && tt.wantToken != "" {
				cached, ok := cache.Get(context.Background(), accessTokenCacheKeyPrefix+"appid")
				assert.True(t, ok)
				assert.Equal(t, tt.wantToken, cached)
			}
		})
	}
}
