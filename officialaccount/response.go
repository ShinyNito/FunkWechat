package officialaccount

import (
	"encoding/json"

	"github.com/ShinyNito/FunkWechat/core"
)

// Response 公众号 API 响应封装
type Response[T any] struct {
	Body      []byte
	Data      T
	errParsed bool
	err       error
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
func (r *Response[T]) Error() error {
	if r.errParsed {
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
	r.err = nil
	return nil
}

// IsSuccess 判断响应是否成功
func (r *Response[T]) IsSuccess() bool {
	return r.Error() == nil
}

// DecodeInto 解析响应到指定的变量
func (r *Response[T]) DecodeInto(v any) error {
	if err := r.Error(); err != nil {
		return err
	}
	return json.Unmarshal(r.Body, v)
}
