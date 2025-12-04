package miniprogram

import (
	"context"
	"fmt"
)

const (
	// Code2SessionURL code2session 接口地址
	Code2SessionPath = "/sns/jscode2session"
)

// Code2SessionRequest code2session 请求参数
type Code2SessionRequest struct {
	// JSCode 登录时获取的 code，可通过 wx.login 获取
	JSCode string
}

// Code2SessionResponse code2session 响应结果
type Code2SessionResponse struct {
	// OpenID 用户唯一标识
	OpenID string `json:"openid"`
	// SessionKey 会话密钥
	SessionKey string `json:"session_key"`
	// UnionID 用户在开放平台的唯一标识符
	// 若当前小程序已绑定到微信开放平台帐号下会返回
	UnionID string `json:"unionid,omitempty"`
}

// Code2Session 通过登录凭证 code 获取 session_key 和 openid
// 接口文档: https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/user-login/code2Session.html
//
// 参数:
//   - ctx: 上下文
//   - req: 请求参数，包含 JSCode
//
// 返回:
//   - *Code2SessionResponse: 响应结果，包含 openid, session_key, unionid 等
//   - error: 可能的错误
//
// 错误:
//   - 40029: code 无效（js_code 无效）
//   - 45011: API 调用太频繁，请稍候再试
//   - 40226: code 被封禁（高风险等级用户，小程序登录拦截）
//   - -1: 系统繁忙，此时请开发者稍候再试
//
// 示例:
//
//	resp, err := mp.Code2Session(ctx, &miniprogram.Code2SessionRequest{
//	    JSCode: "081aBZ000X0pJt1WjY200zWDKK1aBZ0J",
//	})
//	if err != nil {
//	    // 处理错误（包括微信 API 错误）
//	    return err
//	}
//	fmt.Println("OpenID:", resp.OpenID)
//	fmt.Println("SessionKey:", resp.SessionKey)
func (mp *MiniProgram) Code2Session(ctx context.Context, req *Code2SessionRequest) (*Code2SessionResponse, error) {
	if req.JSCode == "" {
		return nil, fmt.Errorf("js_code is required")
	}

	params := map[string]string{
		"appid":      mp.config.AppID,
		"secret":     mp.config.AppSecret,
		"js_code":    req.JSCode,
		"grant_type": "authorization_code",
	}

	mp.config.Logger.DebugContext(ctx, "code2session request",
		"path", Code2SessionPath,
		"params", params,
	)

	result := &Code2SessionResponse{}
	err := mp.GetWithoutToken(ctx, Code2SessionPath, params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type GetPhoneNumberRequest struct {
	// code 是通过 wx.getPhoneNumber 获取到的用户手机号对应的 code
	Code string `json:"code"`
}

// GetPhoneNumberResponse 获取用户手机号响应结果
type GetPhoneNumberResponse struct {
	// PhoneInfo 用户手机号信息
	PhoneInfo PhoneInfo `json:"phone_info"`
}

type PhoneInfo struct {
	// PhoneNumber 用户手机号
	PhoneNumber string `json:"phoneNumber"`
	// PurePhoneNumber 没有区号的手机号
	PurePhoneNumber string `json:"purePhoneNumber"`
	//CountryCode 区号
	CountryCode int `json:"countryCode"`
	// Watermark 水印
	Watermark Watermark `json:"watermark"`
}

type Watermark struct {
	// AppID 小程序 AppID
	AppID string `json:"appid"`
	// Timestamp 获取手机号操作的时间戳
	Timestamp int64 `json:"timestamp"`
}

// GetPhoneNumber 该接口用于将code换取用户手机号。 说明，每个code只能使用一次，code的有效期为5min。
// 接口文档: https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/user-info/phone-number/getPhoneNumber.html
//
// 参数:
//   - ctx: 上下文
//   - GetPhoneNumberRequest : 请求参数
//
// 返回:
//   - *GetPhoneNumberResponse: 响应结果
//   - error: 可能的错误
func (mp *MiniProgram) GetPhoneNumber(ctx context.Context, req *GetPhoneNumberRequest) (*GetPhoneNumberResponse, error) {
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	params := map[string]string{
		"code": req.Code,
	}

	mp.config.Logger.DebugContext(ctx, "get phone number request",
		"path", "/wxa/business/getuserphonenumber",
		"params", params,
	)

	result := &GetPhoneNumberResponse{}
	err := mp.Post(ctx, "/wxa/business/getuserphonenumber", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
