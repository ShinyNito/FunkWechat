package core

import (
	"errors"
	"testing"
)

func TestDecodeWechat(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}

	t.Run("success", func(t *testing.T) {
		got, err := DecodeWechat[sample](200, []byte(`{"errcode":0,"name":"ok"}`))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Name != "ok" {
			t.Fatalf("unexpected name: %s", got.Name)
		}
	})

	t.Run("wechat error on 200", func(t *testing.T) {
		_, err := DecodeWechat[sample](200, []byte(`{"errcode":40001,"errmsg":"invalid token"}`))
		var we *WechatError
		if !errors.As(err, &we) {
			t.Fatalf("expected WechatError, got %v", err)
		}
		if we.ErrCode != 40001 {
			t.Fatalf("unexpected errcode: %d", we.ErrCode)
		}
	})

	t.Run("wechat error on non2xx", func(t *testing.T) {
		_, err := DecodeWechat[sample](401, []byte(`{"errcode":40001,"errmsg":"invalid token"}`))
		var we *WechatError
		if !errors.As(err, &we) {
			t.Fatalf("expected WechatError, got %v", err)
		}
	})

	t.Run("http status error", func(t *testing.T) {
		_, err := DecodeWechat[sample](500, []byte(`{"message":"oops"}`))
		if err == nil || err.Error() == "" {
			t.Fatal("expected http status error")
		}
	})
}
