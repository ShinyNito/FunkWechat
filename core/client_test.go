package core

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) GetToken(ctx context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func (s stubTokenProvider) RefreshToken(ctx context.Context) (string, error) {
	return s.token, s.err
}

func TestClient_DoRequest(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		path          string
		query         map[string]string
		token         string
		tokenErr      error
		setupServer   func(t *testing.T) *httptest.Server
		wantStatus    int
		wantBody      string
		wantQueryKeep map[string]string
		wantErrSub    string
	}{
		{
			name:    "adds access token and preserves caller query",
			baseURL: "",
			path:    "/api",
			query:   map[string]string{"foo": "bar"},
			token:   "token123",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/api", r.URL.Path)
					assert.Equal(t, "bar", r.URL.Query().Get("foo"))
					assert.Equal(t, "token123", r.URL.Query().Get("access_token"))
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"ok":true}`))
				}))
			},
			wantStatus:    http.StatusOK,
			wantBody:      `{"ok":true}`,
			wantQueryKeep: map[string]string{"foo": "bar"},
		},
		{
			name:       "token provider error",
			baseURL:    "",
			path:       "/api",
			tokenErr:   assert.AnError,
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Fatalf("server should not be called")
				}))
			},
			wantErrSub: "get access token",
		},
		{
			name:    "invalid base url",
			baseURL: "http://[::1]:named",
			path:    "/api",
			token:   "t",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			},
			wantErrSub: "parse base url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer(t)
			defer server.Close()

			baseURL := tt.baseURL
			if baseURL == "" {
				baseURL = server.URL
			}

			client := NewClient(
				stubTokenProvider{token: tt.token, err: tt.tokenErr},
				WithBaseURL(baseURL),
				WithHTTPClient(server.Client()),
			)

			respBody, err := client.doRequest(context.Background(), http.MethodGet, tt.path, tt.query, nil)
			if tt.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSub)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, string(respBody))
			assert.NotContains(t, tt.query, "access_token")
			if tt.wantQueryKeep != nil {
				for k, v := range tt.wantQueryKeep {
					assert.Equal(t, v, tt.query[k])
				}
			}
		})
	}
}

func TestClient_Upload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/upload", r.URL.Path)
		require.Equal(t, "token123", r.URL.Query().Get("access_token"))

		err := r.ParseMultipartForm(1024)
		require.NoError(t, err)

		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, file)
		assert.Equal(t, "test.txt", header.Filename)
		assert.Equal(t, "hello", buf.String())
		assert.Equal(t, "extra", r.FormValue("extra_field"))

		_, _ = w.Write([]byte(`{"uploaded":true}`))
	}))
	defer server.Close()

	client := NewClient(
		stubTokenProvider{token: "token123"},
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	body, err := client.Upload(
		context.Background(),
		"/upload",
		"file",
		"test.txt",
		strings.NewReader("hello"),
		map[string]string{"extra_field": "extra"},
	)
	require.NoError(t, err)
	assert.Equal(t, `{"uploaded":true}`, string(body))
}
