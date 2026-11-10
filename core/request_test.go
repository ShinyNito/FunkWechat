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
	"time"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

func fixedToken(value string) *credential.Credential {
	return credential.New(nil, "token", func(context.Context) (string, time.Duration, error) {
		return value, time.Hour, nil
	}, nil)
}

func newTestClient(t *testing.T, handler http.HandlerFunc, token *credential.Credential) *client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := newClient(server.URL, nil, nil)
	c.token = token
	return c
}

func TestRequestGetDecodesJSON(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") != "token" || r.URL.Query().Get("openid") != "o123" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "nickname": "alice"})
	}, fixedToken("token"))

	type resp struct {
		Status
		Nickname string `json:"nickname"`
	}
	got, err := client.request("/cgi-bin/user/info").Query("openid", "o123").Get[resp](context.Background())
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
	}, fixedToken("token"))

	type resp struct {
		Status
		OpenID string `json:"openid"`
	}
	got, err := client.request("/sns/jscode2session").WithoutToken().Get[resp](context.Background())
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

	_, err := client.request("/api").Body(map[string]string{"text": "a&b<c>"}).Post[Status](context.Background())
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
		Status
		MediaID string `json:"media_id"`
	}
	got, err := client.request("/cgi-bin/media/upload").
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
	_, err := client.request("/api").Body(map[string]string{}).Field("k", "v").Post[Status](context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestWechatErrorIncludesPath(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errcode":40013,"errmsg":"invalid appid"}`))
	}, nil)

	_, err := client.request("/api/x").Get[Status](context.Background())
	wechatErr, ok := errors.AsType[*WechatError](err)
	if !ok || wechatErr.ErrCode != ErrInvalidAppID.ErrCode || wechatErr.Path != "/api/x" {
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

	_, err := client.request("/api").Get[Status](context.Background())
	httpErr, ok := errors.AsType[*HTTPError](err)
	if !ok || httpErr.StatusCode != http.StatusBadGateway || httpErr.Path != "/api" || httpErr.Body != "bad gateway" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(err.Error(), "http status 502") || !IsRetryable(context.Background(), err) {
		t.Fatalf("err = %v, retryable = %v", err, IsRetryable(context.Background(), err))
	}
}

func TestRequestDecodesResponseWithoutErrcode(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"openid":"o1","note":"errcode is not a key here"}`))
	}, nil)

	got, err := client.request("/api").Get[struct {
		Status
		OpenID string `json:"openid"`
	}](context.Background())
	if err != nil || got.OpenID != "o1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRequestDebugLogRedactsPersonalData(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":0,"openid":"OPENID-VALUE","unionid":"UNIONID-VALUE","phone_info":{"phoneNumber":"+8613800138000","purePhoneNumber":"13800138000","countryCode":"86"}}`))
	}))
	defer server.Close()
	client := newClient(server.URL, nil, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	_, err := client.request("/api").
		Body(map[string]string{"touser": "TOUSER-VALUE", "template_id": "tpl"}).
		Post[Status](context.Background())
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	out := logs.String()
	for _, leaked := range []string{"OPENID-VALUE", "UNIONID-VALUE", "13800138000", "TOUSER-VALUE"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("log leaks %s: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "tpl") || !strings.Contains(out, "86") {
		t.Fatalf("log dropped non-sensitive fields: %s", out)
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

	raw, err := client.request("/wxa/getwxacodeunlimit").Body(map[string]string{}).PostRaw(context.Background())
	if err != nil || raw.ContentType != "image/png" || !bytes.Equal(raw.Body, png) {
		t.Fatalf("raw = %+v, %v", raw, err)
	}

	_, err = client.request("/wxa/getwxacodeunlimit").Query("fail", "1").PostRaw(context.Background())
	if wechatErr, ok := errors.AsType[*WechatError](err); !ok || wechatErr.ErrCode != 41030 {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestEmptyBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	if _, err := client.request("/api").Get[struct {
		Status
		A int
	}](context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestRejectsOversizedResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxResponseBytes+1))
	}, nil)
	_, err := client.request("/api").GetRaw(context.Background())
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestNetworkErrorRedactsSecrets(t *testing.T) {
	client := newClient("http://127.0.0.1:1", nil, nil)
	client.token = fixedToken("TOKEN-VALUE")

	_, err := client.request("/sns/jscode2session").
		Query("secret", "SECRET-VALUE").
		Query("js_code", "CODE-VALUE").
		Get[Status](context.Background())
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
	client := newClient(server.URL, nil, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	_, err := client.request(StableTokenPath).
		Body(map[string]string{"appid": "app", "secret": "SECRET-VALUE"}).
		WithoutToken().
		Post[Status](context.Background())
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

// selfDecoding 自己实现 UnmarshalJSON 的响应类型：收到完整的响应体，自己负责填充 Status。
type selfDecoding struct {
	Status
	raw string
}

func (s *selfDecoding) UnmarshalJSON(b []byte) error {
	s.raw = string(b)
	return json.Unmarshal(b, &s.Status)
}

func TestRequestSupportsCustomUnmarshaler(t *testing.T) {
	body := `{"errcode":0,"errmsg":"ok","openid":"o1"}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}, nil)
	ctx := context.Background()

	got, err := client.request("/api").Get[selfDecoding](ctx)
	if err != nil || got.raw != body {
		t.Fatalf("got %q, %v", got.raw, err)
	}

	body = `{"errcode":40029,"errmsg":"invalid code"}`
	if _, err := client.request("/api").Get[selfDecoding](ctx); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("err = %v; want ErrInvalidCode", err)
	}
}

func TestRequestResultCarriesNoStatusOnSuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}, nil)

	got, err := client.request("/api").Post[Status](context.Background())
	if err != nil || got.ErrCode != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestRequestReturnsWechatErrorWhenPayloadShapeDiffers(t *testing.T) {
	// 出错时 data 的类型和成功时不同，errcode 还排在它后面，仍然要得到 WechatError。
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":"unavailable","errmsg":"invalid code","errcode":40029}`))
	}, nil)

	_, err := client.request("/api").Get[struct {
		Status
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}](context.Background())
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("err = %v; want ErrInvalidCode", err)
	}
}

func TestRequestDecodeError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>maintenance</html>`))
	}, nil)

	_, err := client.request("/api").Get[Status](context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode /api response") {
		t.Fatalf("err = %v", err)
	}
}
