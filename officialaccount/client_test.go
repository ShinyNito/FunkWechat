package officialaccount

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

func TestGetTicket(t *testing.T) {
	tokenCalls := 0
	ticketCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case accessTokenPath:
			tokenCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case getTicketPath:
			ticketCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode":    0,
				"errmsg":     "ok",
				"ticket":     "ticket-1",
				"expires_in": 7200,
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

	resp, err := client.GetTicket(context.Background(), GetTicketRequest{Type: TicketTypeJSAPI})
	if err != nil {
		t.Fatalf("get ticket: %v", err)
	}
	if resp.Ticket != "ticket-1" {
		t.Fatalf("unexpected ticket: %s", resp.Ticket)
	}

	// cache hit
	resp, err = client.GetTicket(context.Background(), GetTicketRequest{Type: TicketTypeJSAPI})
	if err != nil {
		t.Fatalf("get ticket from cache: %v", err)
	}
	if resp.Ticket != "ticket-1" {
		t.Fatalf("unexpected cached ticket: %s", resp.Ticket)
	}
	if tokenCalls != 1 {
		t.Fatalf("expected 1 token call, got %d", tokenCalls)
	}
	if ticketCalls != 1 {
		t.Fatalf("expected 1 ticket call, got %d", ticketCalls)
	}
}

func TestGetJssdkSign(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case accessTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case getTicketPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "ticket": "ticket-1", "expires_in": 7200})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	sign, err := client.GetJssdkSign(context.Background(), JssdkSignRequest{URL: "https://example.com/a#hash"})
	if err != nil {
		t.Fatalf("get jssdk sign: %v", err)
	}
	if sign.AppID != "appid" {
		t.Fatalf("unexpected appid: %s", sign.AppID)
	}
	if sign.Signature == "" {
		t.Fatal("signature should not be empty")
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
