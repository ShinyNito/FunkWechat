package core

import (
	"net/url"
	"testing"
)

func TestRedactQueryMap(t *testing.T) {
	in := map[string]string{
		"access_token": "token123",
		"secret":       "secret123",
		"normal":       "value",
	}

	out := RedactQueryMap(in)

	if out["access_token"] != "***" {
		t.Fatalf("expected access_token to be redacted, got %q", out["access_token"])
	}
	if out["secret"] != "***" {
		t.Fatalf("expected secret to be redacted, got %q", out["secret"])
	}
	if out["normal"] != "value" {
		t.Fatalf("expected normal to remain unchanged, got %q", out["normal"])
	}

	// 原始 map 不应被修改
	if in["access_token"] != "token123" {
		t.Fatalf("expected input map unchanged")
	}
}

func TestRedactURLQuery(t *testing.T) {
	raw := "https://api.weixin.qq.com/cgi-bin/token?secret=abc&grant_type=client_credential&access_token=tok"
	redacted := RedactURLQuery(raw)

	parsed, err := url.Parse(redacted)
	if err != nil {
		t.Fatalf("parse redacted url: %v", err)
	}

	if parsed.Query().Get("secret") != "***" {
		t.Fatalf("expected secret to be redacted")
	}
	if parsed.Query().Get("access_token") != "***" {
		t.Fatalf("expected access_token to be redacted")
	}
	if parsed.Query().Get("grant_type") != "client_credential" {
		t.Fatalf("expected grant_type to remain unchanged")
	}
}
