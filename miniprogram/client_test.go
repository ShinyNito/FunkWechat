package miniprogram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewValidation(t *testing.T) {
	_, err := New(Config{AppSecret: "secret"})
	if err == nil {
		t.Fatal("expected appid validation error")
	}

	_, err = New(Config{AppID: "appid"})
	if err == nil {
		t.Fatal("expected appsecret validation error")
	}
}

func TestCode2Session(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case Code2SessionPath:
			if got := r.URL.Query().Get("access_token"); got != "" {
				t.Fatalf("unexpected access_token: %s", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"openid":      "openid-1",
				"session_key": "sk-1",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Code2Session(context.Background(), Code2SessionRequest{JSCode: "code"})
	if err != nil {
		t.Fatalf("code2session: %v", err)
	}
	if resp.OpenID != "openid-1" {
		t.Fatalf("unexpected openid: %s", resp.OpenID)
	}
}

func TestGetPhoneNumber(t *testing.T) {
	tokenCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case accessTokenPath:
			tokenCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case "/wxa/business/getuserphonenumber":
			if got := r.URL.Query().Get("access_token"); got != "token-1" {
				t.Fatalf("expected access_token=token-1, got %s", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0,
				"phone_info": map[string]any{
					"phoneNumber":     "13800000000",
					"purePhoneNumber": "13800000000",
					"countryCode":     86,
					"watermark": map[string]any{
						"appid":     "appid",
						"timestamp": 1,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.GetPhoneNumber(context.Background(), GetPhoneNumberRequest{Code: "123"})
	if err != nil {
		t.Fatalf("get phone number: %v", err)
	}
	if resp.PhoneInfo.PhoneNumber != "13800000000" {
		t.Fatalf("unexpected phone: %s", resp.PhoneInfo.PhoneNumber)
	}
	if resp.PhoneInfo.CountryCode != "86" {
		t.Fatalf("unexpected country code: %s", resp.PhoneInfo.CountryCode)
	}
	if tokenCalls != 1 {
		t.Fatalf("expected 1 token call, got %d", tokenCalls)
	}
}

func TestTypedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case accessTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case "/cgi-bin/message/send":
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "msgid": 123})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	type sendResp struct {
		MsgID int `json:"msgid"`
	}
	resp, err := Request[sendResp](client).
		Path("/cgi-bin/message/send").
		Body(map[string]any{"msg": "hello"}).
		Post(context.Background())
	if err != nil {
		t.Fatalf("typed request post: %v", err)
	}
	if resp.MsgID != 123 {
		t.Fatalf("unexpected msgid: %d", resp.MsgID)
	}
}
