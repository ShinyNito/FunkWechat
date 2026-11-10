package wxcrypto

import (
	"testing"
)

func TestSHA1Sign(t *testing.T) {
	tests := []struct {
		name   string
		params []string
		want   string
	}{
		{
			name:   "sorted automatically",
			params: []string{"nonce", "token", "timestamp"},
			want:   "6db4861c77e0633e0105672fcd41c9fc2766e26e",
		},
		{
			name:   "single param",
			params: []string{"abc"},
			want:   "a9993e364706816aba3e25717850c26c9cd0d89d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA1Sign(tt.params...)
			if got != tt.want {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestSHA256Sign(t *testing.T) {
	got := SHA256Sign("c", "a", "b")
	if got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("got %v; want %v", got, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
	}
}

func TestHMACSHA256(t *testing.T) {
	got := HMACSHA256("hello", "key")
	if got != "9307b3b915efb5171ff14d8cb55fbcc798c6c0ef1456d66ded1a6aa723a58b7b" {
		t.Errorf("got %v; want %v", got, "9307b3b915efb5171ff14d8cb55fbcc798c6c0ef1456d66ded1a6aa723a58b7b")
	}
}

func TestVerifySignature(t *testing.T) {
	if !(VerifySignature("6db4861c77e0633e0105672fcd41c9fc2766e26e", "timestamp", "nonce", "token")) {
		t.Errorf("expected true")
	}
	if VerifySignature("invalid", "timestamp", "nonce", "token") {
		t.Errorf("expected false")
	}
}

func TestVerifyMsgSignature(t *testing.T) {
	if !(VerifyMsgSignature("aa7fc06a800892bacf85c0ce5a37f057dbe560ca", "ts", "nonce", "token", "encrypted")) {
		t.Errorf("expected true")
	}
	if VerifyMsgSignature("invalid", "ts", "nonce", "token", "encrypted") {
		t.Errorf("expected false")
	}
}

func TestSHA1SignDoesNotModifyInput(t *testing.T) {
	params := []string{"nonce", "token", "timestamp"}
	_ = SHA1Sign(params...)
	if params[0] != "nonce" || params[1] != "token" || params[2] != "timestamp" {
		t.Fatalf("params modified: %v", params)
	}
}

func TestSignJSSDK(t *testing.T) {
	// 微信 JS-SDK 文档附录 1 的示例。
	got := SignJSSDK("sM4AOVdWfPE4DxkXGEs8VMCPGGVi4C3VM0P37wVUCFvkVAy_90u5h9nbSlYy3-Sl-HhTdfl2fzFy1AOcHKP7qg",
		"Wm3WZYTPz0wzccnW", 1414587457, "http://mp.weixin.qq.com?params=value")
	if got != "0f9de62fce790f9a083d5c99e95740ceb90c27ed" {
		t.Fatalf("signature = %s", got)
	}
}
