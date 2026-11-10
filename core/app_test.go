package core

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAppValidation(t *testing.T) {
	if _, err := NewApp(AppConfig{AppSecret: "secret"}); err == nil {
		t.Fatal("expected appid validation error")
	}
	if _, err := NewApp(AppConfig{AppID: "appid"}); err == nil {
		t.Fatal("expected appsecret validation error")
	}
}

func TestAppAccessTokenAndLogs(t *testing.T) {
	var tokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case StableTokenPath:
			tokenCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "expires_in": 7200})
		case "/api":
			if r.URL.Query().Get("access_token") != "token-1" {
				t.Errorf("access_token = %q", r.URL.Query().Get("access_token"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	app, err := NewApp(AppConfig{
		AppID:     "wx123",
		AppSecret: "secret",
		BaseURL:   server.URL,
		Logger:    slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	ctx := context.Background()

	if token, err := app.AccessToken(ctx); err != nil || token != "token-1" {
		t.Fatalf("access token = %q, %v", token, err)
	}
	if _, err := app.Request("/api").Get[struct{}](ctx); err != nil {
		t.Fatalf("request: %v", err)
	}
	if tokenCalls != 1 {
		t.Fatalf("token calls = %d; want 1", tokenCalls)
	}
	for line := range strings.Lines(strings.TrimSpace(logs.String())) {
		if !strings.Contains(line, "appid=wx123") {
			t.Fatalf("log line without appid: %s", line)
		}
	}
}
