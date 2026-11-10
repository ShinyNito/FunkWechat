package officialaccount

import (
	"context"
	"fmt"

	"github.com/ShinyNito/FunkWechat/v3/core"
)

const (
	getTicketPath = "/cgi-bin/ticket/getticket"
	// 2.2.0 及以前的缓存键前缀为 officialaccount:jsapi_ticket:，值是带过期时间的 JSON。
	// 换用新前缀，避免与旧版本共用缓存时把旧格式的值当作 ticket 返回。
	ticketCacheKeyPrefix = "officialaccount:ticket:"
)

type TicketType string

const (
	TicketTypeJSAPI  TicketType = "jsapi"
	TicketTypeWxCard TicketType = "wx_card"
)

type GetTicketRequest struct {
	// Type 为空时使用 TicketTypeJSAPI。
	Type TicketType
}

type getTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

// GetTicket 获取 ticket，优先读缓存。
func (c *Client) GetTicket(ctx context.Context, req GetTicketRequest) (string, error) {
	manager, err := c.ticketManager(req.Type)
	if err != nil {
		return "", err
	}
	return manager.GetToken(ctx)
}

// RefreshTicket 跳过缓存，重新向微信获取 ticket。
func (c *Client) RefreshTicket(ctx context.Context, ticketType TicketType) (string, error) {
	manager, err := c.ticketManager(ticketType)
	if err != nil {
		return "", err
	}
	return manager.RefreshToken(ctx)
}

// ticketManager 返回指定类型的 ticket 管理器。每种类型各自缓存、各自合并并发刷新。
func (c *Client) ticketManager(ticketType TicketType) (*core.TokenManager, error) {
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}

	c.ticketMu.Lock()
	defer c.ticketMu.Unlock()
	if manager, ok := c.tickets[ticketType]; ok {
		return manager, nil
	}

	manager, err := core.NewTokenManager(core.TokenManagerConfig{
		Cache:    c.cfg.Cache,
		CacheKey: c.ticketCacheKey(ticketType),
		Logger:   c.cfg.Logger,
		Fetcher: func(ctx context.Context) (core.TokenFetchResult, error) {
			resp, err := c.apiClient.Request(getTicketPath).
				Query("type", string(ticketType)).
				Get[getTicketResponse](ctx)
			if err != nil {
				return core.TokenFetchResult{}, fmt.Errorf("get ticket: %w", err)
			}
			return core.TokenFetchResult{Token: resp.Ticket, ExpiresIn: resp.ExpiresIn}, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new ticket manager: %w", err)
	}
	c.tickets[ticketType] = manager
	return manager, nil
}

func (c *Client) ticketCacheKey(ticketType TicketType) string {
	return ticketCacheKeyPrefix + c.cfg.AppID + ":" + string(ticketType)
}
