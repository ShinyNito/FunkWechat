package officialaccount

import (
	"context"
	"fmt"
	"time"
)

const (
	getTicketPath             = "/cgi-bin/ticket/getticket"
	jsapiTicketCacheKeyPrefix = "officialaccount:jsapi_ticket:"
	ticketExpireBuffer        = 300
)

type TicketType string

const (
	TicketTypeJSAPI  TicketType = "jsapi"
	TicketTypeWxCard TicketType = "wx_card"
)

type GetTicketRequest struct {
	Type TicketType
}

type GetTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

func (c *Client) GetTicket(ctx context.Context, req GetTicketRequest) (GetTicketResponse, error) {
	ticketType := req.Type
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}

	cacheKey := c.ticketCacheKey(ticketType)
	if ticket, ok := c.cfg.Cache.Get(ctx, cacheKey); ok {
		return GetTicketResponse{Ticket: ticket, ExpiresIn: 7200}, nil
	}
	return c.refreshTicket(ctx, ticketType)
}

func (c *Client) RefreshTicket(ctx context.Context, ticketType TicketType) (GetTicketResponse, error) {
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}
	return c.refreshTicket(ctx, ticketType)
}

func (c *Client) refreshTicket(ctx context.Context, ticketType TicketType) (GetTicketResponse, error) {
	c.ticketMu.Lock()
	defer c.ticketMu.Unlock()

	cacheKey := c.ticketCacheKey(ticketType)
	if ticket, ok := c.cfg.Cache.Get(ctx, cacheKey); ok {
		return GetTicketResponse{Ticket: ticket, ExpiresIn: 7200}, nil
	}

	resp, err := Request[GetTicketResponse](c).
		Path(getTicketPath).
		Query("type", string(ticketType)).
		Get(ctx)
	if err != nil {
		return GetTicketResponse{}, fmt.Errorf("get ticket: %w", err)
	}

	ttlSeconds := max(resp.ExpiresIn-ticketExpireBuffer, 1)
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := c.cfg.Cache.Set(ctx, cacheKey, resp.Ticket, ttl); err != nil {
		c.cfg.Logger.WarnContext(ctx, "cache ticket failed", "type", ticketType, "error", err)
	}

	return resp, nil
}

func (c *Client) ticketCacheKey(ticketType TicketType) string {
	return jsapiTicketCacheKeyPrefix + c.cfg.AppID + ":" + string(ticketType)
}
