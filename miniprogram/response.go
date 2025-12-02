package miniprogram

import (
	"encoding/json"

	"github.com/ShinyNito/FunkWechat/core"
)

// Response 小程序 API 响应封装
type Response struct {
	// Body 原始响应体
	Body []byte
}

// NewResponse 创建响应实例
func NewResponse(body []byte) *Response {
	return &Response{Body: body}
}

// String 返回响应字符串
func (r *Response) String() string {
	return string(r.Body)
}

// JSON 解析响应到指定结构体
func (r *Response) JSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Map 解析响应为 map
func (r *Response) Map() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Error 检查响应是否包含微信错误
// 如果响应不是有效 JSON，返回 ResponseParseError
// 如果 errcode != 0，返回 WechatError
func (r *Response) Error() error {
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(r.Body, &result); err != nil {
		// 非 JSON 响应视为错误（可能是网关错误、HTML 错误页等）
		return core.NewResponseParseError(r.Body, err)
	}
	if result.ErrCode != 0 {
		return core.NewWechatError(result.ErrCode, result.ErrMsg)
	}
	return nil
}

// IsSuccess 判断响应是否成功
func (r *Response) IsSuccess() bool {
	return r.Error() == nil
}
