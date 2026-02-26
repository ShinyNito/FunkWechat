package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type staticTokenProvider struct {
	token string
	err   error
}

func (s *staticTokenProvider) GetToken(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func (s *staticTokenProvider) RefreshToken(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func newTestClient(t *testing.T, server *httptest.Server, tokenProvider AccessTokenProvider) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		BaseURL:       server.URL,
		TokenProvider: tokenProvider,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestTypedRequestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "token" {
			t.Fatalf("missing access token")
		}
		if r.URL.Query().Get("openid") != "o123" {
			t.Fatalf("missing openid")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "nickname": "alice"})
	}))
	defer server.Close()

	client := newTestClient(t, server, &staticTokenProvider{token: "token"})

	type resp struct {
		Nickname string `json:"nickname"`
	}
	got, err := NewTypedRequest[resp](client).
		Path("/cgi-bin/user/info").
		Query("openid", "o123").
		Get(context.Background())
	if err != nil {
		t.Fatalf("typed get: %v", err)
	}
	if got.Nickname != "alice" {
		t.Fatalf("unexpected nickname: %s", got.Nickname)
	}
}

func TestTypedRequestWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "" {
			t.Fatal("access token should be omitted")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"openid": "test", "session_key": "key"})
	}))
	defer server.Close()

	client := newTestClient(t, server, &staticTokenProvider{token: "token"})

	type resp struct {
		OpenID string `json:"openid"`
	}
	got, err := NewTypedRequest[resp](client).
		Path("/sns/jscode2session").
		WithoutToken().
		Get(context.Background())
	if err != nil {
		t.Fatalf("typed get: %v", err)
	}
	if got.OpenID != "test" {
		t.Fatalf("unexpected openid: %s", got.OpenID)
	}
}

func TestTypedRequestUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.FormValue("type") != "image" {
			t.Fatalf("unexpected field type")
		}
		f, header, err := r.FormFile("media")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer f.Close()
		if header.Filename != "a.jpg" {
			t.Fatalf("unexpected filename: %s", header.Filename)
		}
		data, _ := io.ReadAll(f)
		if string(data) != "hello" {
			t.Fatalf("unexpected content: %s", string(data))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "media_id": "m1"})
	}))
	defer server.Close()

	client := newTestClient(t, server, &staticTokenProvider{token: "token"})

	type uploadResp struct {
		MediaID string `json:"media_id"`
	}
	resp, err := NewTypedRequest[uploadResp](client).
		Path("/cgi-bin/media/upload").
		UploadFile("media", "a.jpg", bytes.NewReader([]byte("hello"))).
		UploadField("type", "image").
		Post(context.Background())
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.MediaID != "m1" {
		t.Fatalf("unexpected media id: %s", resp.MediaID)
	}
}
