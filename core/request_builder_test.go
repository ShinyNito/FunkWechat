package core

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestBuilder_Query(t *testing.T) {
	tests := []struct {
		name          string
		buildRequest  func(*RequestBuilder) *RequestBuilder
		validateQuery func(t *testing.T, r *http.Request)
	}{
		{
			name: "单个查询参数",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.Query("key", "value")
			},
			validateQuery: func(t *testing.T, r *http.Request) {
				if got := r.URL.Query().Get("key"); got != "value" {
					t.Errorf("expected key=value, got key=%s", got)
				}
			},
		},
		{
			name: "多个查询参数（链式）",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.Query("a", "1").Query("b", "2").Query("c", "3")
			},
			validateQuery: func(t *testing.T, r *http.Request) {
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
			},
		},
		{
			name: "QueryMap 批量设置",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.QueryMap(map[string]string{
					"x": "10",
					"y": "20",
					"z": "30",
				})
			},
			validateQuery: func(t *testing.T, r *http.Request) {
				params := []struct{ key, value string }{
					{"x", "10"},
					{"y", "20"},
					{"z", "30"},
				}
				for _, p := range params {
					if got := r.URL.Query().Get(p.key); got != p.value {
						t.Errorf("expected %s=%s, got %s=%s", p.key, p.value, p.key, got)
					}
				}
			},
		},
		{
			name: "Query 和 QueryMap 混合使用",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.Query("a", "1").
					QueryMap(map[string]string{"b": "2", "c": "3"}).
					Query("d", "4")
			},
			validateQuery: func(t *testing.T, r *http.Request) {
				params := []struct{ key, value string }{
					{"a", "1"},
					{"b", "2"},
					{"c", "3"},
					{"d", "4"},
				}
				for _, p := range params {
					if got := r.URL.Query().Get(p.key); got != p.value {
						t.Errorf("expected %s=%s, got %s=%s", p.key, p.value, p.key, got)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.validateQuery(t, r)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(&mockTokenProvider{token: "test"}),
				WithBaseURL(server.URL),
			)

			builder := client.Request().Path("/test")
			builder = tt.buildRequest(builder)

			_, err := builder.Get(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRequestBuilder_WithToken(t *testing.T) {
	tests := []struct {
		name          string
		buildRequest  func(*RequestBuilder) *RequestBuilder
		wantToken     bool
		tokenProvider AccessTokenProvider
	}{
		{
			name: "默认带 token",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b
			},
			wantToken:     true,
			tokenProvider: &mockTokenProvider{token: "default_token"},
		},
		{
			name: "显式设置带 token",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.WithToken()
			},
			wantToken:     true,
			tokenProvider: &mockTokenProvider{token: "explicit_token"},
		},
		{
			name: "不带 token",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.WithoutToken()
			},
			wantToken:     false,
			tokenProvider: &mockTokenProvider{token: "should_not_see"},
		},
		{
			name: "先设置 WithToken 再设置 WithoutToken",
			buildRequest: func(b *RequestBuilder) *RequestBuilder {
				return b.WithToken().WithoutToken()
			},
			wantToken:     false,
			tokenProvider: &mockTokenProvider{token: "should_not_see"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := r.URL.Query().Get("access_token")
				if tt.wantToken {
					if token == "" {
						t.Error("expected access_token in query, got none")
					}
				} else {
					if token != "" {
						t.Errorf("expected no access_token, got %s", token)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(tt.tokenProvider),
				WithBaseURL(server.URL),
			)

			builder := client.Request().Path("/test")
			builder = tt.buildRequest(builder)

			_, err := builder.Get(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRequestBuilder_UploadFile(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		fileName    string
		fileContent string
		extraFields map[string]string
		validateReq func(t *testing.T, r *http.Request)
	}{
		{
			name:        "基本文件上传",
			fieldName:   "media",
			fileName:    "test.jpg",
			fileContent: "fake image content",
			validateReq: func(t *testing.T, r *http.Request) {
				// 验证 Content-Type
				contentType := r.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "multipart/form-data") {
					t.Errorf("expected multipart/form-data, got %s", contentType)
				}

				// 解析 multipart
				err := r.ParseMultipartForm(32 << 20)
				if err != nil {
					t.Fatalf("failed to parse multipart form: %v", err)
				}

				// 验证文件
				file, header, err := r.FormFile("media")
				if err != nil {
					t.Fatalf("failed to get form file: %v", err)
				}
				defer file.Close()

				if header.Filename != "test.jpg" {
					t.Errorf("expected filename=test.jpg, got %s", header.Filename)
				}

				content, _ := io.ReadAll(file)
				if string(content) != "fake image content" {
					t.Errorf("expected content='fake image content', got %s", string(content))
				}
			},
		},
		{
			name:        "文件上传带额外字段",
			fieldName:   "media",
			fileName:    "test.png",
			fileContent: "png data",
			extraFields: map[string]string{
				"type":        "image",
				"description": "test image",
			},
			validateReq: func(t *testing.T, r *http.Request) {
				err := r.ParseMultipartForm(32 << 20)
				if err != nil {
					t.Fatalf("failed to parse multipart: %v", err)
				}

				// 验证额外字段
				if r.FormValue("type") != "image" {
					t.Errorf("expected type=image, got %s", r.FormValue("type"))
				}
				if r.FormValue("description") != "test image" {
					t.Errorf("expected description='test image', got %s", r.FormValue("description"))
				}

				// 验证文件存在
				_, _, err = r.FormFile("media")
				if err != nil {
					t.Errorf("failed to get form file: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 验证方法是 POST
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}

				tt.validateReq(t, r)

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"errcode":0,"media_id":"123"}`))
			}))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(&mockTokenProvider{token: "test"}),
				WithBaseURL(server.URL),
			)

			builder := client.Request().
				Path("/cgi-bin/media/upload").
				UploadFile(tt.fieldName, tt.fileName, bytes.NewReader([]byte(tt.fileContent)))

			if len(tt.extraFields) > 0 {
				builder = builder.UploadExtraFields(tt.extraFields)
			}

			body, err := builder.Post(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证响应
			if !strings.Contains(string(body), "media_id") {
				t.Errorf("expected media_id in response, got %s", string(body))
			}
		})
	}
}

func TestRequestBuilder_Body(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		validateReq func(t *testing.T, r *http.Request)
		wantErr     bool
	}{
		{
			name: "map 作为请求体",
			body: map[string]any{
				"message": "hello",
				"code":    123,
			},
			validateReq: func(t *testing.T, r *http.Request) {
				// 验证 Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("expected Content-Type=application/json, got %s", ct)
				}

				// 验证请求体
				bodyBytes, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(bodyBytes), "hello") {
					t.Errorf("expected body to contain 'hello', got %s", string(bodyBytes))
				}
			},
		},
		{
			name: "struct 作为请求体",
			body: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "test",
				Age:  25,
			},
			validateReq: func(t *testing.T, r *http.Request) {
				bodyBytes, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(bodyBytes), "test") {
					t.Errorf("expected body to contain 'test', got %s", string(bodyBytes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.validateReq(t, r)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(&mockTokenProvider{token: "test"}),
				WithBaseURL(server.URL),
			)

			_, err := client.Request().
				Path("/test").
				Body(tt.body).
				Post(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRequestBuilder_PathValidation(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "正常路径",
			path:    "/cgi-bin/token",
			wantErr: false,
		},
		{
			name:    "带前导斜杠的路径",
			path:    "/test/path",
			wantErr: false,
		},
		{
			name:    "不带前导斜杠的路径",
			path:    "test/path",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(
				WithTokenProvider(&mockTokenProvider{token: "test"}),
				WithBaseURL(server.URL),
			)

			_, err := client.Request().
				Path(tt.path).
				Get(context.Background())

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
