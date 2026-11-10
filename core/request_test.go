package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type staticTokenProvider struct {
	token string
}

func (s *staticTokenProvider) GetToken(context.Context) (string, error)     { return s.token, nil }
func (s *staticTokenProvider) RefreshToken(context.Context) (string, error) { return s.token, nil }

func newTestClient(t *testing.T, handler http.HandlerFunc, token AccessTokenProvider) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient(ClientConfig{BaseURL: server.URL, TokenProvider: token})
}

func TestRequestGetDecodesJSON(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "token" || r.URL.Query().Get("openid") != "o123" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "nickname": "alice"})
	}, &staticTokenProvider{token: "token"})

	type resp struct {
		Nickname string `json:"nickname"`
	}
	got, err := client.Request("/cgi-bin/user/info").Query("openid", "o123").Get[resp](context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Nickname != "alice" {
		t.Fatalf("nickname = %q", got.Nickname)
	}
}

func TestRequestWithoutToken(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("access_token") {
			t.Error("access token should be omitted")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"openid": "test"})
	}, &staticTokenProvider{token: "token"})

	type resp struct {
		OpenID string `json:"openid"`
	}
	got, err := client.Request("/sns/jscode2session").WithoutToken().Get[resp](context.Background())
	if err != nil || got.OpenID != "test" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRequestPostJSONDoesNotEscapeHTML(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"text":"a&b<c>"}` {
			t.Errorf("body = %s", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0})
	}, nil)

	_, err := client.Request("/api").Body(map[string]string{"text": "a&b<c>"}).Post[struct{}](context.Background())
	if err != nil {
		t.Fatalf("post: %v", err)
	}
}

func TestRequestMultipart(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.FormValue("type") != "image" {
			t.Errorf("type = %q", r.FormValue("type"))
		}
		f, header, err := r.FormFile("media")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		if header.Filename != "a.jpg" || string(data) != "hello" {
			t.Errorf("file = %s %q", header.Filename, data)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"media_id": "m1"})
	}, nil)

	type resp struct {
		MediaID string `json:"media_id"`
	}
	got, err := client.Request("/cgi-bin/media/upload").
		File("media", "a.jpg", []byte("hello")).
		Field("type", "image").
		Post[resp](context.Background())
	if err != nil || got.MediaID != "m1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRequestRejectsJSONBodyWithMultipart(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("request should not be sent")
	}, nil)
	_, err := client.Request("/api").Body(map[string]string{}).Field("k", "v").Post[struct{}](context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestWechatErrorIncludesPath(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errcode":40013,"errmsg":"invalid appid"}`))
	}, nil)

	_, err := client.Request("/api/x").Get[struct{}](context.Background())
	wechatErr, ok := errors.AsType[*WechatError](err)
	if !ok || wechatErr.ErrCode != ErrCodeInvalidAppID || wechatErr.Path != "/api/x" {
		t.Fatalf("err = %v", err)
	}
	if err.Error() != "wechat /api/x: [40013] invalid appid" {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestRequestHTTPStatusError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}, nil)

	_, err := client.Request("/api").Get[struct{}](context.Background())
	if err == nil || !strings.Contains(err.Error(), "http status 502") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestRaw(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nimage")
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") != "" {
			_, _ = w.Write([]byte(`{"errcode":41030,"errmsg":"invalid page"}`))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}, nil)

	raw, err := client.Request("/wxa/getwxacodeunlimit").Body(map[string]string{}).PostRaw(context.Background())
	if err != nil || raw.ContentType != "image/png" || !bytes.Equal(raw.Body, png) {
		t.Fatalf("raw = %+v, %v", raw, err)
	}

	_, err = client.Request("/wxa/getwxacodeunlimit").Query("fail", "1").PostRaw(context.Background())
	if wechatErr, ok := errors.AsType[*WechatError](err); !ok || wechatErr.ErrCode != 41030 {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestEmptyBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	if _, err := client.Request("/api").Get[struct{ A int }](context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestRejectsOversizedResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxResponseBytes+1))
	}, nil)
	_, err := client.Request("/api").GetRaw(context.Background())
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestNetworkErrorRedactsSecrets(t *testing.T) {
	client := NewClient(ClientConfig{
		BaseURL:       "http://127.0.0.1:1",
		TokenProvider: &staticTokenProvider{token: "TOKEN-VALUE"},
	})

	_, err := client.Request("/sns/jscode2session").
		Query("secret", "SECRET-VALUE").
		Query("js_code", "CODE-VALUE").
		Get[struct{}](context.Background())
	if err == nil {
		t.Fatal("expected network error")
	}
	for _, leaked := range []string{"TOKEN-VALUE", "SECRET-VALUE", "CODE-VALUE"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("error leaks %s: %v", leaked, err)
		}
	}
}

func TestRequestDebugLogIsRedacted(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TOKEN-VALUE", "expires_in": 7200})
	}))
	defer server.Close()
	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		Logger:  slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})

	_, err := client.Request(StableTokenPath).
		Body(map[string]string{"appid": "app", "secret": "SECRET-VALUE"}).
		WithoutToken().
		Post[struct{}](context.Background())
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	out := logs.String()
	if !strings.Contains(out, "path="+StableTokenPath) || !strings.Contains(out, "status=200") {
		t.Fatalf("log missing fields: %s", out)
	}
	for _, leaked := range []string{"TOKEN-VALUE", "SECRET-VALUE"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("log leaks %s: %s", leaked, out)
		}
	}
}
