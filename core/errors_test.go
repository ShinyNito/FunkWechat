package core

import (
	"errors"
	"testing"
)

func TestWechatError(t *testing.T) {
	err := NewWechatError(40001, "invalid")
	if err.Error() != "wechat error: [40001] invalid" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestIsTokenError(t *testing.T) {
	if !IsTokenError(NewWechatError(ErrCodeInvalidToken, "invalid token")) {
		t.Fatal("expected invalid token error")
	}
	if !IsTokenError(NewWechatError(ErrCodeExpiredToken, "expired token")) {
		t.Fatal("expected expired token error")
	}
	if IsTokenError(NewWechatError(ErrCodeInvalidAppID, "invalid appid")) {
		t.Fatal("unexpected token error")
	}
	if IsTokenError(errors.New("plain")) {
		t.Fatal("unexpected token error for plain error")
	}
}
