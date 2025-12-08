package officialaccount

import (
	"context"
	"fmt"
	"time"
)

const (
	// getTicketPath 获取 ticket 的路径
	getTicketPath = "/cgi-bin/ticket/getticket"
	// jsapi ticket 缓存 key 前缀
	jsapiTicketCacheKeyPrefix = "officialaccount:jsapi_ticket:"
	// ticket 提前过期时间（秒）
	ticketExpireBuffer = 300
)

// TicketType ticket 类型
type TicketType string

const (
	// TicketTypeJSAPI JS-SDK 票据
	TicketTypeJSAPI TicketType = "jsapi"
	// TicketTypeWxCard 微信卡券票据
	TicketTypeWxCard TicketType = "wx_card"
)

// GetTicketRequest 获取 ticket 请求参数
type GetTicketRequest struct {
	// Type ticket 类型，默认为 jsapi
	Type TicketType
}

// GetTicketResponse 获取 ticket 响应结果
type GetTicketResponse struct {
	// Ticket 临时票据
	Ticket string `json:"ticket"`
	// ExpiresIn 有效期（秒）
	ExpiresIn int `json:"expires_in"`
}

// GetTicket 获取 JS-SDK 临时票据
// 接口文档: https://developers.weixin.qq.com/doc/offiaccount/OA_Web_Apps/JS-SDK.html#62
//
// Api_ticket 是用于调用 js-sdk 的临时票据，有效期为 7200 秒，通过 access_token 来获取。
//
// 参数:
//   - ctx: 上下文
//   - req: 请求参数，包含 Type（默认 jsapi）
//
// 返回:
//   - *GetTicketResponse: 响应结果，包含 ticket 和 expires_in
//   - error: 可能的错误
//
// 示例:
//
//	resp, err := oa.GetTicket(ctx, &officialaccount.GetTicketRequest{
//	    Type: officialaccount.TicketTypeJSAPI,
//	})
//	if err != nil {
//	    return err
//	}
//	fmt.Println("Ticket:", resp.Ticket)
func (oa *OfficialAccount) GetTicket(ctx context.Context, req *GetTicketRequest) (*GetTicketResponse, error) {
	ticketType := req.Type
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}

	cacheKey := oa.ticketCacheKey(ticketType)

	// 尝试从缓存获取
	if ticket, ok := oa.config.Cache.Get(ctx, cacheKey); ok {
		oa.config.Logger.Debug("ticket from cache",
			"appid", oa.config.AppID,
			"type", ticketType,
		)
		return &GetTicketResponse{Ticket: ticket, ExpiresIn: 7200}, nil
	}

	// 缓存未命中，请求 API
	return oa.refreshTicket(ctx, ticketType)
}

// RefreshTicket 强制刷新 ticket
func (oa *OfficialAccount) RefreshTicket(ctx context.Context, ticketType TicketType) (*GetTicketResponse, error) {
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}
	return oa.refreshTicket(ctx, ticketType)
}

// refreshTicket 刷新 ticket（内部方法，带锁防止并发刷新）
func (oa *OfficialAccount) refreshTicket(ctx context.Context, ticketType TicketType) (*GetTicketResponse, error) {
	oa.ticketMu.Lock()
	defer oa.ticketMu.Unlock()

	cacheKey := oa.ticketCacheKey(ticketType)

	// 双重检查，避免并发刷新
	if ticket, ok := oa.config.Cache.Get(ctx, cacheKey); ok {
		return &GetTicketResponse{Ticket: ticket, ExpiresIn: 7200}, nil
	}

	oa.config.Logger.Info("refreshing ticket",
		"appid", oa.config.AppID,
		"type", ticketType,
	)

	params := map[string]string{
		"type": string(ticketType),
	}

	result := &GetTicketResponse{}
	err := oa.Get(ctx, getTicketPath, params, result)
	if err != nil {
		oa.config.Logger.Error("refresh ticket failed",
			"appid", oa.config.AppID,
			"type", ticketType,
			"error", err,
		)
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	// 缓存 ticket
	ttlSeconds := max(result.ExpiresIn-ticketExpireBuffer, 1)
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := oa.config.Cache.Set(ctx, cacheKey, result.Ticket, ttl); err != nil {
		oa.config.Logger.Warn("cache ticket failed",
			"appid", oa.config.AppID,
			"type", ticketType,
			"error", err,
		)
	}

	oa.config.Logger.Info("ticket refreshed",
		"appid", oa.config.AppID,
		"type", ticketType,
		"expires_in", result.ExpiresIn,
	)

	return result, nil
}

// ticketCacheKey 生成 ticket 缓存 key
func (oa *OfficialAccount) ticketCacheKey(ticketType TicketType) string {
	return jsapiTicketCacheKeyPrefix + oa.config.AppID + ":" + string(ticketType)
}
