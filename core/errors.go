package core

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// ErrInvalidArgument 调用参数不合法，请求没有发出。用 errors.Is 判断。
var ErrInvalidArgument = errors.New("invalid argument")

// WechatError 微信接口返回的业务错误（errcode 非 0）。
//
// 用 errors.Is 和下面的预定义错误按 errcode 判断：
//
//	if errors.Is(err, core.ErrUserRefused) { ... }
type WechatError struct {
	// Path 出错的接口路径，例如 /cgi-bin/message/subscribe/send。
	Path    string
	ErrCode int
	ErrMsg  string
}

func (e *WechatError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("wechat: [%d] %s", e.ErrCode, e.ErrMsg)
	}
	return fmt.Sprintf("wechat %s: [%d] %s", e.Path, e.ErrCode, e.ErrMsg)
}

// Is 只比较 errcode，使 errors.Is(err, ErrUserRefused) 这样的判断成立。
func (e *WechatError) Is(target error) bool {
	t, ok := target.(*WechatError)
	return ok && t.ErrCode == e.ErrCode
}

// 常用 errcode。其他错误码用 errors.AsType[*WechatError] 取出 ErrCode 判断。
var (
	ErrBusy             = &WechatError{ErrCode: -1, ErrMsg: "system busy"}
	ErrInvalidToken     = &WechatError{ErrCode: 40001, ErrMsg: "invalid credential"}
	ErrInvalidAppID     = &WechatError{ErrCode: 40013, ErrMsg: "invalid appid"}
	ErrIllegalToken     = &WechatError{ErrCode: 40014, ErrMsg: "invalid access_token"}
	ErrInvalidCode      = &WechatError{ErrCode: 40029, ErrMsg: "invalid code"}
	ErrInvalidAppSecret = &WechatError{ErrCode: 40125, ErrMsg: "invalid appsecret"}
	ErrCodeAlreadyUsed  = &WechatError{ErrCode: 40163, ErrMsg: "code been used"}
	ErrExpiredToken     = &WechatError{ErrCode: 42001, ErrMsg: "access_token expired"}
	ErrUserRefused      = &WechatError{ErrCode: 43101, ErrMsg: "user refuse to accept the msg"} // 订阅消息中表示未订阅或订阅次数已用完
	ErrFrequencyLimited = &WechatError{ErrCode: 45011, ErrMsg: "api minute-quota reach limit"}
	ErrAPIUnauthorized  = &WechatError{ErrCode: 48001, ErrMsg: "api unauthorized"}
)

// HTTPError 微信返回了非 2xx 状态码，且响应体里没有 errcode。
type HTTPError struct {
	Path       string
	StatusCode int
	// Body 响应体的前 256 字节。
	Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("wechat %s: http status %d: %s", e.Path, e.StatusCode, e.Body)
}

// IsTokenError 报告 err 是否表示 access_token 无效或过期。
// SDK 遇到这类错误会自动刷新 token 并重试一次。
func IsTokenError(err error) bool {
	return errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrIllegalToken) || errors.Is(err, ErrExpiredToken)
}

// IsRetryable 报告用同一个 ctx 稍后重试是否可能成功：微信系统繁忙（errcode -1）、
// HTTP 429 或 5xx、网络超时（例如 HTTPClient 的 Timeout）。
// SDK 不会自动重试这类错误，重试策略由调用方决定。
//
// ctx 应为发起这次调用时使用的 ctx。ctx 已取消或已超时时返回 false：用它重试会立即失败。
// 只看 err 无法区分调用方 ctx 超时和 HTTP 客户端超时，两者都是 context.DeadlineExceeded。
func IsRetryable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, ErrBusy) {
		return true
	}
	if httpErr, ok := errors.AsType[*HTTPError](err); ok {
		return httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	// context.DeadlineExceeded 也实现了 net.Error，SDK 内部获取凭证的超时同样会走到这里。
	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}
	return false
}
