package core

import (
	"errors"
	"testing"
)

func TestWechatError(t *testing.T) {
	err := &WechatError{Path: "/cgi-bin/x", ErrCode: 40001, ErrMsg: "invalid"}
	if err.Error() != "wechat /cgi-bin/x: [40001] invalid" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestIsTokenError(t *testing.T) {
	if !IsTokenError(&WechatError{ErrCode: ErrCodeInvalidToken, ErrMsg: "invalid token"}) {
		t.Fatal("expected invalid token error")
	}
	if !IsTokenError(&WechatError{ErrCode: ErrCodeExpiredToken, ErrMsg: "expired token"}) {
		t.Fatal("expected expired token error")
	}
	if IsTokenError(&WechatError{ErrCode: ErrCodeInvalidAppID, ErrMsg: "invalid appid"}) {
		t.Fatal("unexpected token error")
	}
	if IsTokenError(errors.New("plain")) {
		t.Fatal("unexpected token error for plain error")
	}
}
