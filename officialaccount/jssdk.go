package officialaccount

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ShinyNito/FunkWechat/core/utils"
)

// JssdkSignRequest JS-SDK 签名请求参数
type JssdkSignRequest struct {
	// URL 当前网页的完整 URL（不包含 hash，即去掉 # 及其后面的部分）
	URL string
}

// JssdkSignResponse JS-SDK 签名响应结果
type JssdkSignResponse struct {
	// AppID 公众号 AppID
	AppID string `json:"appId"`
	// Timestamp 时间戳（秒）
	Timestamp int64 `json:"timestamp"`
	// NonceStr 随机字符串
	NonceStr string `json:"nonceStr"`
	// Signature 签名
	Signature string `json:"signature"`
}

// GetJssdkSign 生成 JS-SDK 签名
// 用于前端调用 wx.config 时所需的签名参数
//
// 接口文档: https://developers.weixin.qq.com/doc/offiaccount/OA_Web_Apps/JS-SDK.html#62
//
// 签名算法:
//  1. 获取 jsapi_ticket
//  2. 生成随机字符串 noncestr
//  3. 获取当前时间戳 timestamp
//  4. 按照固定格式拼接字符串并做 SHA1 签名
//
// 参数:
//   - ctx: 上下文
//   - req: 请求参数，包含当前页面 URL
//
// 返回:
//   - *JssdkSignResponse: 响应结果，包含 appId, timestamp, nonceStr, signature
//   - error: 可能的错误
//
// 示例:
//
//	resp, err := oa.GetJssdkSign(ctx, &officialaccount.JssdkSignRequest{
//	    URL: "https://example.com/path",
//	})
//	if err != nil {
//	    return err
//	}
//	// 返回给前端用于 wx.config
//	// {appId, timestamp, nonceStr, signature}
func (oa *OfficialAccount) GetJssdkSign(ctx context.Context, req *JssdkSignRequest) (*JssdkSignResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	// URL 预处理：去掉 hash 部分
	url := req.URL
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	// 1. 获取 jsapi_ticket
	ticketResp, err := oa.GetTicket(ctx, &GetTicketRequest{Type: TicketTypeJSAPI})
	if err != nil {
		return nil, fmt.Errorf("get jsapi_ticket: %w", err)
	}

	// 2. 生成随机字符串
	nonceStr, err := utils.RandomString(16)
	if err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// 3. 获取当前时间戳（秒）
	timestamp := time.Now().Unix()

	// 4. 生成签名
	signature := oa.sign(ticketResp.Ticket, nonceStr, timestamp, url)

	return &JssdkSignResponse{
		AppID:     oa.config.AppID,
		Timestamp: timestamp,
		NonceStr:  nonceStr,
		Signature: signature,
	}, nil
}

// sign 计算 JS-SDK 签名
// 签名格式：jsapi_ticket={ticket}&noncestr={noncestr}&timestamp={timestamp}&url={url}
// 然后对该字符串进行 SHA1 签名
func (oa *OfficialAccount) sign(ticket, nonceStr string, timestamp int64, url string) string {
	// 按照官方文档要求的固定格式拼接
	str := fmt.Sprintf("jsapi_ticket=%s&noncestr=%s&timestamp=%d&url=%s",
		ticket, nonceStr, timestamp, url)

	oa.config.Logger.Debug("jssdk sign string",
		"string", str,
	)

	// SHA1 签名
	h := sha1.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
