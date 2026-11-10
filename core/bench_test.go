package core

import (
	"net/http"
	"testing"
)

type benchResp struct {
	OpenID string `json:"openid"`
}

func BenchmarkDecodeResponse(b *testing.B) {
	resp := response{status: http.StatusOK, body: []byte(`{"errcode":0,"errmsg":"ok","openid":"openid-123456"}`)}
	for b.Loop() {
		if _, err := decodeJSON[benchResp]("/api", resp, resp.check("/api")); err != nil {
			b.Fatal(err)
		}
	}
}
