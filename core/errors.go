package core

import (
	"errors"
	"fmt"
)

// WechatError 微信业务错误（errcode/errmsg）。
type WechatError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (e *WechatError) Error() string {
	return fmt.Sprintf("wechat error: [%d] %s", e.ErrCode, e.ErrMsg)
}

func NewWechatError(code int, msg string) *WechatError {
	return &WechatError{ErrCode: code, ErrMsg: msg}
}

const (
	ErrCodeSuccess          = 0
	ErrCodeBusy             = -1
	ErrCodeInvalidToken     = 40001
	ErrCodeExpiredToken     = 42001
	ErrCodeInvalidAppID     = 40013
	ErrCodeInvalidAppSecret = 40125
	ErrCodeInvalidCode      = 40029
	ErrCodeCodeUsed         = 40163
	ErrCodeFreqLimit        = 45011
	ErrCodeAPIUnauthorized  = 48001
)

func IsTokenError(err error) bool {
	if err == nil {
		return false
	}
	var we *WechatError
	if !errors.As(err, &we) {
		return false
	}
	return we.ErrCode == ErrCodeInvalidToken || we.ErrCode == ErrCodeExpiredToken
}
