package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// mockTokenProvider 模拟 token 提供者
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) RefreshToken(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func (m *mockTokenProvider) GetToken(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func TestClient_Request_GET(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		query          map[string]string
		withToken      bool
		tokenProvider  AccessTokenProvider
		serverHandler  func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrContain string
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:  "成功的 GET 请求（带 token）",
			path:  "/test",
			query: map[string]string{"openid": "test_openid"},
			tokenProvider: &mockTokenProvider{
				token: "test_token",
			},
			withToken: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// 验证请求方法
				if r.Method != http.MethodGet {
					t.Errorf("expected GET method, got %s", r.Method)
				}
				// 验证 access_token
				if token := r.URL.Query().Get("access_token"); token != "test_token" {
					t.Errorf("expected access_token=test_token, got %s", token)
				}
				// 验证查询参数
				if openid := r.URL.Query().Get("openid"); openid != "test_openid" {
					t.Errorf("expected openid=test_openid, got %s", openid)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"errcode": 0,
					"data":    "success",
				})
			},
			validateBody: func(t *testing.T, body []byte) {
				var result map[string]any
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if result["data"] != "success" {
					t.Errorf("expected data=success, got %v", result["data"])
				}
			},
		},
		{
			name:  "GET 请求（不带 token）",
			path:  "/sns/jscode2session",
			query: map[string]string{"js_code": "test_code"},
			tokenProvider: &mockTokenProvider{
				token: "test_token",
			},
			withToken: false,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// 验证没有 access_token
				if token := r.URL.Query().Get("access_token"); token != "" {
					t.Errorf("expected no access_token, got %s", token)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"openid":      "test_openid",
					"session_key": "test_session",
				})
			},
			validateBody: func(t *testing.T, body []byte) {
				var result map[string]any
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["openid"] != "test_openid" {
					t.Errorf("expected openid=test_openid, got %v", result["openid"])
				}
			},
		},
		{
			name:  "Token 提供者返回错误",
			path:  "/test",
			query: map[string]string{},
			tokenProvider: &mockTokenProvider{
				err: errors.New("token provider error"),
			},
			withToken:      true,
			wantErr:        true,
			wantErrContain: "get access token",
		},
		{
			name:  "服务器返回错误状态码",
			path:  "/test",
			query: map[string]string{},
			tokenProvider: &mockTokenProvider{
				token: "test_token",
			},
			withToken: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			validateBody: func(t *testing.T, body []byte) {
				// HTTP 错误不应导致请求失败，应返回响应体
				if !strings.Contains(string(body), "Internal Server Error") {
					t.Errorf("expected error message in body")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = httptest.NewServer(http.HandlerFunc(tt.serverHandler))
				defer server.Close()
			}

			// 创建客户端
			opts := []ClientOption{
				WithTokenProvider(tt.tokenProvider),
			}
			if server != nil {
				opts = append(opts, WithBaseURL(server.URL))
			}
			client := NewClient(opts...)

			// 构建请求
			builder := client.Request().Path(tt.path).QueryMap(tt.query)
			if !tt.withToken {
				builder = builder.WithoutToken()
			}

			// 发送请求
			body, err := builder.Get(context.Background())

			// 验证错误
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("expected error to contain %q, got %q", tt.wantErrContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证响应体
			if tt.validateBody != nil {
				tt.validateBody(t, body)
			}
		})
	}
}

func TestClient_Request_POST(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		query         map[string]string
		body          any
		withToken     bool
		serverHandler func(w http.ResponseWriter, r *http.Request)
		wantErr       bool
		validateBody  func(t *testing.T, body []byte)
	}{
		{
			name:      "成功的 POST JSON 请求",
			path:      "/message/send",
			body:      map[string]any{"message": "test"},
			withToken: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// 验证请求方法
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				// 验证 Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("expected Content-Type=application/json, got %s", ct)
				}
				// 验证请求体
				body, _ := io.ReadAll(r.Body)
				var reqBody map[string]any
				json.Unmarshal(body, &reqBody)
				if reqBody["message"] != "test" {
					t.Errorf("expected message=test, got %v", reqBody["message"])
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"errcode": 0,
					"msgid":   12345,
				})
			},
			validateBody: func(t *testing.T, body []byte) {
				var result map[string]any
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["msgid"].(float64) != 12345 {
					t.Errorf("expected msgid=12345, got %v", result["msgid"])
				}
			},
		},
		{
			name:      "POST 请求（带查询参数）",
			path:      "/message/send",
			query:     map[string]string{"type": "text"},
			body:      map[string]any{"content": "hello"},
			withToken: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// 验证查询参数
				if r.URL.Query().Get("type") != "text" {
					t.Errorf("expected type=text")
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"errcode": 0})
			},
			validateBody: func(t *testing.T, body []byte) {
				var result map[string]any
				json.Unmarshal(body, &result)
				if result["errcode"].(float64) != 0 {
					t.Errorf("expected errcode=0")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(&mockTokenProvider{token: "test_token"}),
				WithBaseURL(server.URL),
			)

			builder := client.Request().
				Path(tt.path).
				QueryMap(tt.query).
				Body(tt.body)
			if !tt.withToken {
				builder = builder.WithoutToken()
			}

			body, err := builder.Post(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validateBody != nil {
				tt.validateBody(t, body)
			}
		})
	}
}

func TestClient_Request_ChainedQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证所有查询参数
		params := []struct{ key, value string }{
			{"a", "1"},
			{"b", "2"},
			{"c", "3"},
		}
		for _, p := range params {
			if got := r.URL.Query().Get(p.key); got != p.value {
				t.Errorf("expected %s=%s, got %s=%s", p.key, p.value, p.key, got)
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"errcode": 0})
	}))
	defer server.Close()

	client := NewClient(
		WithTokenProvider(&mockTokenProvider{token: "test_token"}),
		WithBaseURL(server.URL),
	)

	_, err := client.Request().
		Path("/test").
		Query("a", "1").
		Query("b", "2").
		Query("c", "3").
		Get(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_BuildURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		path        string
		query       map[string]string
		wantPath    string
		wantQueries map[string]string // 期望的查询参数
		wantErr     bool
	}{
		{
			name:     "简单路径",
			baseURL:  "https://api.weixin.qq.com",
			path:     "/cgi-bin/token",
			query:    map[string]string{"grant_type": "client_credential"},
			wantPath: "/cgi-bin/token",
			wantQueries: map[string]string{
				"grant_type": "client_credential",
			},
		},
		{
			name:     "路径包含查询参数",
			baseURL:  "https://api.weixin.qq.com",
			path:     "/test?foo=bar",
			query:    map[string]string{"a": "1"},
			wantPath: "/test",
			wantQueries: map[string]string{
				"foo": "bar",
				"a":   "1",
			},
		},
		{
			name:        "没有查询参数",
			baseURL:     "https://api.weixin.qq.com",
			path:        "/test",
			query:       nil,
			wantPath:    "/test",
			wantQueries: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(WithBaseURL(tt.baseURL))
			gotURL, err := client.buildURL(tt.path, tt.query)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 解析返回的 URL
			parsedURL, err := url.Parse(gotURL)
			if err != nil {
				t.Fatalf("failed to parse result URL: %v", err)
			}

			// 验证路径
			if parsedURL.Path != tt.wantPath {
				t.Errorf("expected path=%q, got %q", tt.wantPath, parsedURL.Path)
			}

			// 验证查询参数（不依赖顺序）
			gotQueries := parsedURL.Query()
			for key, wantValue := range tt.wantQueries {
				if gotValue := gotQueries.Get(key); gotValue != wantValue {
					t.Errorf("expected query %s=%q, got %s=%q", key, wantValue, key, gotValue)
				}
			}

			// 验证没有额外的查询参数
			if len(gotQueries) != len(tt.wantQueries) {
				t.Errorf("expected %d query params, got %d: %v", len(tt.wantQueries), len(gotQueries), gotQueries)
			}
		})
	}
}

func TestClient_NilTokenProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证没有 access_token
		if token := r.URL.Query().Get("access_token"); token != "" {
			t.Errorf("expected no access_token when tokenProvider is nil")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	// 创建没有 tokenProvider 的客户端
	client := NewClient(WithBaseURL(server.URL))

	// 即使请求带 token，也不应该报错（因为 tokenProvider 是 nil）
	_, err := client.Request().
		Path("/test").
		WithToken().
		Get(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
