package core

import "testing"

type benchResp struct {
	Status
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
}

func BenchmarkDecodeResponse(b *testing.B) {
	resp := response{status: 200, body: []byte(`{"errcode":0,"errmsg":"ok","openid":"oABCD1234567890","session_key":"c2Vzc2lvbmtleQ==","unionid":"u1234567890"}`)}
	for b.Loop() {
		var out benchResp
		if err := decodeResponse("/x", resp, &out); err != nil || out.OpenID == "" {
			b.Fatal(err)
		}
	}
}
