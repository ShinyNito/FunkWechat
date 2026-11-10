package core

import (
	"errors"
	"fmt"
)

// WechatError 微信接口返回的业务错误（errcode 非 0）。
type WechatError struct {
	// Path 出错的接口路径，例如 /cgi-bin/message/subscribe/send。
	Path    string
	ErrCode int
	ErrMsg  string
}

func (e *WechatError) Error() string {
	return fmt.Sprintf("wechat %s: [%d] %s", e.Path, e.ErrCode, e.ErrMsg)
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
	ErrCodeUserRefused      = 43101 // 用户拒绝接收消息；订阅消息中表示未订阅或订阅次数已用完
)

func IsTokenError(err error) bool {
	if err == nil {
		return false
	}
	we, ok := errors.AsType[*WechatError](err)
	if !ok {
		return false
	}
	return we.ErrCode == ErrCodeInvalidToken || we.ErrCode == ErrCodeExpiredToken
}
