package miniprogram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShinyNito/FunkWechat/v3/core"
)

func newSubscribeTestClient(t *testing.T, handle func(w http.ResponseWriter, body map[string]any)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case core.StableTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case SendSubscribeMessagePath:
			if got := r.URL.Query().Get("access_token"); got != "token-1" {
				t.Errorf("access_token = %q; want token-1", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
			}
			handle(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestSendSubscribeMessage(t *testing.T) {
	client := newSubscribeTestClient(t, func(w http.ResponseWriter, body map[string]any) {
		if body["touser"] != "openid-1" || body["template_id"] != "tpl-1" || body["miniprogram_state"] != "developer" {
			t.Errorf("unexpected body: %v", body)
		}
		data, _ := body["data"].(map[string]any)
		thing1, _ := data["thing1"].(map[string]any)
		if thing1["value"] != "盲盒" {
			t.Errorf("unexpected data: %v", body["data"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
	})

	err := client.SendSubscribeMessage(context.Background(), SubscribeMessageRequest{
		ToUser:           "openid-1",
		TemplateID:       "tpl-1",
		Page:             "/pages/index",
		Data:             map[string]SubscribeMessageValue{"thing1": {Value: "盲盒"}},
		MiniProgramState: MiniProgramStateDeveloper,
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestSendSubscribeMessageUserRefused(t *testing.T) {
	client := newSubscribeTestClient(t, func(w http.ResponseWriter, _ map[string]any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": core.ErrCodeUserRefused, "errmsg": "user refuse to accept the msg"})
	})

	err := client.SendSubscribeMessage(context.Background(), SubscribeMessageRequest{ToUser: "openid-1", TemplateID: "tpl-1"})
	var wechatErr *core.WechatError
	if !errors.As(err, &wechatErr) || wechatErr.ErrCode != core.ErrCodeUserRefused {
		t.Fatalf("expected user refused error, got %v", err)
	}
}

func TestSendSubscribeMessageValidation(t *testing.T) {
	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.SendSubscribeMessage(context.Background(), SubscribeMessageRequest{TemplateID: "tpl"}); err == nil {
		t.Fatal("expected touser validation error")
	}
	if err := client.SendSubscribeMessage(context.Background(), SubscribeMessageRequest{ToUser: "openid"}); err == nil {
		t.Fatal("expected template_id validation error")
	}
}
