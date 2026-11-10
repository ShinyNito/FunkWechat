package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	raw := "https://api.weixin.qq.com/cgi-bin/token?secret=abc&grant_type=client_credential&access_token=tok"
	redacted := redactURL(raw)

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

func TestRedactBody(t *testing.T) {
	body := []byte(`{"appid":"app","secret":"sec","data":{"session_key":"sk","list":[{"access_token":"tok","n":1}]}}`)

	got := redactBody(body)

	for _, leaked := range []string{`"sec"`, `"sk"`, `"tok"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %s to be redacted, got %s", leaked, got)
		}
	}
	for _, kept := range []string{`"appid":"app"`, `"n":1`} {
		if !strings.Contains(got, kept) {
			t.Fatalf("expected %s to be kept, got %s", kept, got)
		}
	}
}

func TestRedactBodyNonJSON(t *testing.T) {
	if got := redactBody([]byte("secret=abc")); got != "<non-JSON body, 10 bytes>" {
		t.Fatalf("unexpected output: %s", got)
	}
}
