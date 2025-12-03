package miniprogram

import (
	"encoding/json"

	"github.com/ShinyNito/FunkWechat/core"
)

// Response 小程序 API 响应封装
type Response[T any] struct {
	Body      []byte
	Data      T
	errParsed bool  // 是否已经解析过
	err       error // 解析出的错误（nil 也缓存）
}

// NewResponse 创建响应实例
func NewResponse[T any](body []byte) *Response[T] {
	return &Response[T]{Body: body}
}

// String 返回响应字符串
func (r *Response[T]) String() string {
	return string(r.Body)
}

// Decode 一次性完成：
// 1. 先探测 errcode/errmsg
// 2. 再解析到业务模型 T
func (r *Response[T]) Decode() (T, error) {
	var zero T

	if err := r.Error(); err != nil {
		return zero, err
	}
	// 再解析成业务模型
	var out T
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return zero, err
	}
	return out, nil
}

// Map 解析响应为 map
func (r *Response[T]) Map() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Error 检查响应是否包含微信错误
// 如果响应不是有效 JSON，返回 ResponseParseError
// 如果 errcode != 0，返回 WechatError
func (r *Response[T]) Error() error {
	if r.errParsed { // 已经解析过，直接复用
		return r.err
	}
	r.errParsed = true

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(r.Body, &result); err != nil {
		r.err = core.NewResponseParseError(r.Body, err)
		return r.err
	}
	if result.ErrCode != 0 {
		r.err = core.NewWechatError(result.ErrCode, result.ErrMsg)
		return r.err
	}
	// 成功的情况也要缓存 nil
	r.err = nil
	return nil
}

// IsSuccess 判断响应是否成功
func (r *Response[T]) IsSuccess() bool {
	return r.Error() == nil
}

// DecodeInto 解析响应到指定的变量（用于非泛型场景）
// 先检查微信错误，再解析到目标变量
//
// 示例:
//
//	var userInfo UserInfo
//	resp := NewResponse[any](body)
//	err := resp.DecodeInto(&userInfo)
func (r *Response[T]) DecodeInto(v any) error {
	// 先探测错误
	if err := r.Error(); err != nil {
		return err
	}
	// 再解析到目标变量
	return json.Unmarshal(r.Body, v)
}
