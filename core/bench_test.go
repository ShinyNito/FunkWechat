package core

import "testing"

type benchResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	OpenID  string `json:"openid"`
}

func BenchmarkDecodeWechat(b *testing.B) {
	body := []byte(`{"errcode":0,"errmsg":"ok","openid":"openid-123456"}`)
	for b.Loop() {
		_, err := DecodeWechat[benchResp](200, body)
		if err != nil {
			b.Fatal(err)
		}
	}
}
