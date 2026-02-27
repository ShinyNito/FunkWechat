package officialaccount

import (
	"context"
	"encoding/json"
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

type cachedTicket struct {
	Ticket    string `json:"ticket"`
	ExpiresAt int64  `json:"expires_at"`
}

type getTicketAPIResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

func (c *Client) GetTicket(ctx context.Context, req GetTicketRequest) (string, error) {
	ticketType := req.Type
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}

	cacheKey := c.ticketCacheKey(ticketType)
	if raw, ok := c.cfg.Cache.Get(ctx, cacheKey); ok {
		if ticket, ok := decodeCachedTicket(raw); ok {
			return ticket, nil
		}
		if err := c.cfg.Cache.Delete(ctx, cacheKey); err != nil {
			c.cfg.Logger.WarnContext(ctx, "delete invalid cached ticket failed", "type", ticketType, "error", err)
		}
	}
	return c.refreshTicket(ctx, ticketType)
}

func (c *Client) RefreshTicket(ctx context.Context, ticketType TicketType) (string, error) {
	if ticketType == "" {
		ticketType = TicketTypeJSAPI
	}
	return c.refreshTicket(ctx, ticketType)
}

func (c *Client) refreshTicket(ctx context.Context, ticketType TicketType) (string, error) {
	c.ticketMu.Lock()
	defer c.ticketMu.Unlock()

	cacheKey := c.ticketCacheKey(ticketType)
	if raw, ok := c.cfg.Cache.Get(ctx, cacheKey); ok {
		if ticket, ok := decodeCachedTicket(raw); ok {
			return ticket, nil
		}
		if err := c.cfg.Cache.Delete(ctx, cacheKey); err != nil {
			c.cfg.Logger.WarnContext(ctx, "delete invalid cached ticket failed", "type", ticketType, "error", err)
		}
	}

	resp, err := Request[getTicketAPIResponse](c).
		Path(getTicketPath).
		Query("type", string(ticketType)).
		Get(ctx)
	if err != nil {
		return "", fmt.Errorf("get ticket: %w", err)
	}
	if resp.Ticket == "" {
		return "", fmt.Errorf("empty ticket in response")
	}

	ttlSeconds := max(resp.ExpiresIn-ticketExpireBuffer, 1)
	ttl := time.Duration(ttlSeconds) * time.Second

	cacheValue, err := json.Marshal(cachedTicket{
		Ticket:    resp.Ticket,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		c.cfg.Logger.WarnContext(ctx, "marshal cached ticket failed", "type", ticketType, "error", err)
		return resp.Ticket, nil
	}

	if err := c.cfg.Cache.Set(ctx, cacheKey, string(cacheValue), ttl); err != nil {
		c.cfg.Logger.WarnContext(ctx, "cache ticket failed", "type", ticketType, "error", err)
	}

	return resp.Ticket, nil
}

func (c *Client) ticketCacheKey(ticketType TicketType) string {
	return jsapiTicketCacheKeyPrefix + c.cfg.AppID + ":" + string(ticketType)
}

func decodeCachedTicket(raw string) (string, bool) {
	var cached cachedTicket
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		return "", false
	}
	if cached.Ticket == "" {
		return "", false
	}
	if cached.ExpiresAt > 0 && time.Now().Unix() >= cached.ExpiresAt {
		return "", false
	}
	return cached.Ticket, true
}
