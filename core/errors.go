package core

import (
	"errors"
	"fmt"
)

// WechatError 微信 API 错误
type WechatError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// Error 实现 error 接口
func (e *WechatError) Error() string {
	return fmt.Sprintf("wechat error: [%d] %s", e.ErrCode, e.ErrMsg)
}

// IsSuccess 判断是否成功（errcode 为 0 表示成功）
func (e *WechatError) IsSuccess() bool {
	return e.ErrCode == 0
}

// NewWechatError 创建微信错误
func NewWechatError(code int, msg string) *WechatError {
	return &WechatError{
		ErrCode: code,
		ErrMsg:  msg,
	}
}

// 常见错误码定义
const (
	ErrCodeSuccess          = 0     // 成功
	ErrCodeBusy             = -1    // 系统繁忙
	ErrCodeInvalidToken     = 40001 // access_token 无效
	ErrCodeExpiredToken     = 42001 // access_token 过期
	ErrCodeInvalidAppID     = 40013 // 无效的 AppID
	ErrCodeInvalidAppSecret = 40125 // 无效的 AppSecret
	ErrCodeInvalidCode      = 40029 // 无效的 code
	ErrCodeCodeUsed         = 40163 // code 已被使用
	ErrCodeFreqLimit        = 45011 // 频率限制
	ErrCodeAPIUnauthorized  = 48001 // API 未授权
)

// IsTokenError 判断是否为 token 相关错误（需要刷新 token）
func IsTokenError(err error) bool {
	if we, ok := errors.AsType[*WechatError](err); ok {
		return we.ErrCode == ErrCodeInvalidToken || we.ErrCode == ErrCodeExpiredToken
	}
	return false
}

// ResponseParseError 响应解析错误
// 当响应体不是有效的 JSON 时返回此错误
type ResponseParseError struct {
	Body []byte // 原始响应体
	Err  error  // 底层解析错误
}

// Error 实现 error 接口
func (e *ResponseParseError) Error() string {
	return fmt.Sprintf("failed to parse response: %v", e.Err)
}

// Unwrap 支持 errors.Is/As
func (e *ResponseParseError) Unwrap() error {
	return e.Err
}

// NewResponseParseError 创建响应解析错误
func NewResponseParseError(body []byte, err error) *ResponseParseError {
	return &ResponseParseError{
		Body: body,
		Err:  err,
	}
}
