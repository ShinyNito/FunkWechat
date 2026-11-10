package officialaccount

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShinyNito/FunkWechat/v3/core"
)

func TestNewValidation(t *testing.T) {
	_, err := New(core.AppConfig{AppSecret: "secret"})
	if err == nil {
		t.Fatal("expected appid validation error")
	}
	_, err = New(core.AppConfig{AppID: "appid"})
	if err == nil {
		t.Fatal("expected appsecret validation error")
	}
}

func TestGetTicket(t *testing.T) {
	tokenCalls := 0
	ticketCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case core.StableTokenPath:
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

	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ticket, err := client.GetTicket(context.Background(), TicketTypeJSAPI)
	if err != nil {
		t.Fatalf("get ticket: %v", err)
	}
	if ticket != "ticket-1" {
		t.Fatalf("unexpected ticket: %s", ticket)
	}

	// cache hit
	ticket, err = client.GetTicket(context.Background(), TicketTypeJSAPI)
	if err != nil {
		t.Fatalf("get ticket from cache: %v", err)
	}
	if ticket != "ticket-1" {
		t.Fatalf("unexpected cached ticket: %s", ticket)
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
		case core.StableTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case getTicketPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "ticket": "ticket-1", "expires_in": 7200})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
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
		case core.StableTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case "/cgi-bin/message/send":
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "msgid": 123})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	type sendResp struct {
		core.Status
		MsgID int `json:"msgid"`
	}
	resp, err := client.Request("/cgi-bin/message/send").
		Body(map[string]any{"msg": "hello"}).
		Post[sendResp](context.Background())
	if err != nil {
		t.Fatalf("typed request post: %v", err)
	}
	if resp.MsgID != 123 {
		t.Fatalf("unexpected msgid: %d", resp.MsgID)
	}
}

func newTicketTestServer(t *testing.T, ticketCalls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case core.StableTokenPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case getTicketPath:
			*ticketCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode":    0,
				"ticket":     fmt.Sprintf("%s-%d", r.URL.Query().Get("type"), *ticketCalls),
				"expires_in": 7200,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRefreshTicketBypassesCache(t *testing.T) {
	ticketCalls := 0
	server := newTicketTestServer(t, &ticketCalls)
	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := context.Background()

	if _, err := client.GetTicket(ctx, TicketTypeJSAPI); err != nil {
		t.Fatalf("get ticket: %v", err)
	}
	ticket, err := client.RefreshTicket(ctx, TicketTypeJSAPI)
	if err != nil {
		t.Fatalf("refresh ticket: %v", err)
	}
	if ticket != "jsapi-2" {
		t.Fatalf("ticket = %q; want jsapi-2", ticket)
	}
	cached, err := client.GetTicket(ctx, TicketTypeJSAPI)
	if err != nil || cached != "jsapi-2" {
		t.Fatalf("cached ticket = %q, %v; want jsapi-2", cached, err)
	}
}

func TestTicketTypesAreCachedSeparately(t *testing.T) {
	ticketCalls := 0
	server := newTicketTestServer(t, &ticketCalls)
	client, err := New(core.AppConfig{AppID: "appid", AppSecret: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := context.Background()

	jsapi, err := client.GetTicket(ctx, TicketTypeJSAPI)
	if err != nil {
		t.Fatalf("get jsapi ticket: %v", err)
	}
	card, err := client.GetTicket(ctx, TicketTypeWxCard)
	if err != nil {
		t.Fatalf("get wx_card ticket: %v", err)
	}
	if jsapi != "jsapi-1" || card != "wx_card-2" || ticketCalls != 2 {
		t.Fatalf("jsapi = %q, wx_card = %q, calls = %d", jsapi, card, ticketCalls)
	}
}
