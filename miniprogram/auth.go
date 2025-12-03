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

	mp.config.Logger.Debug("code2session request",
		"path", Code2SessionPath,
		"params", params,
	)

	result := Code2SessionResponse{}
	err := mp.GetWithoutToken(ctx, Code2SessionPath, params, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
