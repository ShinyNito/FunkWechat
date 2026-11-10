package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// rotatingTokenProvider 首次返回 old，刷新后返回 new。
type rotatingTokenProvider struct {
	refreshed  atomic.Bool
	refreshes  atomic.Int32
	refreshErr error
}

func (p *rotatingTokenProvider) GetToken(context.Context) (string, error) {
	if p.refreshed.Load() {
		return "new", nil
	}
	return "old", nil
}

func (p *rotatingTokenProvider) RefreshToken(context.Context) (string, error) {
	p.refreshes.Add(1)
	if p.refreshErr != nil {
		return "", p.refreshErr
	}
	p.refreshed.Store(true)
	return "new", nil
}

// newTokenCheckingServer 只接受 access_token=valid 的请求，其余返回 40001。
func newTokenCheckingServer(t *testing.T, valid string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("access_token") != valid {
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": ErrCodeInvalidToken, "errmsg": "invalid credential"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "value": "ok"})
	}))
	t.Cleanup(server.Close)
	return server
}

type valueResp struct {
	Value string `json:"value"`
}

func TestTypedRequestRefreshesTokenOnceOnTokenError(t *testing.T) {
	var calls atomic.Int32
	server := newTokenCheckingServer(t, "new", &calls)
	provider := &rotatingTokenProvider{}
	client := newTestClient(t, server, provider)

	got, err := NewTypedRequest[valueResp](client).
		Path("/api").
		Body(map[string]string{"k": "v"}).
		Post(context.Background())
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if got.Value != "ok" {
		t.Fatalf("unexpected value: %q", got.Value)
	}
	if calls.Load() != 2 || provider.refreshes.Load() != 1 {
		t.Fatalf("calls = %d, refreshes = %d; want 2 and 1", calls.Load(), provider.refreshes.Load())
	}
}

func TestTypedRequestRetriesOnlyOnce(t *testing.T) {
	var calls atomic.Int32
	server := newTokenCheckingServer(t, "never", &calls)
	provider := &rotatingTokenProvider{}
	client := newTestClient(t, server, provider)

	_, err := NewTypedRequest[valueResp](client).Path("/api").Get(context.Background())
	if !IsTokenError(err) {
		t.Fatalf("expected token error, got %v", err)
	}
	if calls.Load() != 2 || provider.refreshes.Load() != 1 {
		t.Fatalf("calls = %d, refreshes = %d; want 2 and 1", calls.Load(), provider.refreshes.Load())
	}
}

func TestTypedRequestReturnsBothErrorsWhenRefreshFails(t *testing.T) {
	var calls atomic.Int32
	server := newTokenCheckingServer(t, "new", &calls)
	refreshErr := errors.New("refresh failed")
	provider := &rotatingTokenProvider{refreshErr: refreshErr}
	client := newTestClient(t, server, provider)

	_, err := NewTypedRequest[valueResp](client).Path("/api").Get(context.Background())
	if !IsTokenError(err) || !errors.Is(err, refreshErr) {
		t.Fatalf("expected token error joined with refresh error, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d; want 1", calls.Load())
	}
}

func TestTypedRequestDoesNotRetryUpload(t *testing.T) {
	var calls atomic.Int32
	server := newTokenCheckingServer(t, "new", &calls)
	provider := &rotatingTokenProvider{}
	client := newTestClient(t, server, provider)

	_, err := NewTypedRequest[valueResp](client).
		Path("/upload").
		UploadFile("media", "a.jpg", bytes.NewReader([]byte("hello"))).
		Post(context.Background())
	if !IsTokenError(err) {
		t.Fatalf("expected token error, got %v", err)
	}
	if calls.Load() != 1 || provider.refreshes.Load() != 0 {
		t.Fatalf("calls = %d, refreshes = %d; want 1 and 0", calls.Load(), provider.refreshes.Load())
	}
}

func TestTypedRequestWithoutTokenDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	server := newTokenCheckingServer(t, "new", &calls)
	provider := &rotatingTokenProvider{}
	client := newTestClient(t, server, provider)

	_, err := NewTypedRequest[valueResp](client).Path("/api").WithoutToken().Get(context.Background())
	if !IsTokenError(err) {
		t.Fatalf("expected token error, got %v", err)
	}
	if calls.Load() != 1 || provider.refreshes.Load() != 0 {
		t.Fatalf("calls = %d, refreshes = %d; want 1 and 0", calls.Load(), provider.refreshes.Load())
	}
}

func TestStableTokenFetcher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != StableTokenPath {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("credentials must not be sent in query: %s", r.URL.RawQuery)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		want := map[string]any{"grant_type": "client_credential", "appid": "app", "secret": "sec", "force_refresh": false}
		for key, value := range want {
			if body[key] != value {
				t.Errorf("body[%s] = %v; want %v", key, body[key], value)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "stable", "expires_in": 7200})
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	got, err := StableTokenFetcher(client, "app", "sec")(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.Token != "stable" || got.ExpiresIn != 7200 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestStableTokenFetcherReturnsWechatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": ErrCodeInvalidAppSecret, "errmsg": "invalid appsecret"})
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	_, err := StableTokenFetcher(client, "app", "bad")(context.Background())
	var wechatErr *WechatError
	if !errors.As(err, &wechatErr) || wechatErr.ErrCode != ErrCodeInvalidAppSecret {
		t.Fatalf("expected appsecret error, got %v", err)
	}
	if strings.Contains(err.Error(), "bad") {
		t.Fatalf("error leaks secret: %v", err)
	}
}
