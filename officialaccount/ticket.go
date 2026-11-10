package officialaccount

import (
	"context"
	"fmt"
	"time"

	"github.com/ShinyNito/FunkWechat/v3/internal/credential"
)

const (
	getTicketPath        = "/cgi-bin/ticket/getticket"
	ticketCacheKeyPrefix = "funkwechat:ticket:"
)

type TicketType string

const (
	TicketTypeJSAPI  TicketType = "jsapi"
	TicketTypeWxCard TicketType = "wx_card"
)

type getTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

func newTicketCredentials(c *Client) map[TicketType]*credential.Credential {
	tickets := make(map[TicketType]*credential.Credential, 2)
	for _, ticketType := range []TicketType{TicketTypeJSAPI, TicketTypeWxCard} {
		key := ticketCacheKeyPrefix + c.Config().AppID + ":" + string(ticketType)
		tickets[ticketType] = credential.New(c.Config().Cache, key, c.fetchTicket(ticketType), c.Logger())
	}
	return tickets
}

func (c *Client) fetchTicket(ticketType TicketType) credential.FetchFunc {
	return func(ctx context.Context) (string, time.Duration, error) {
		resp, err := c.Request(getTicketPath).
			Query("type", string(ticketType)).
			Get[getTicketResponse](ctx)
		if err != nil {
			return "", 0, fmt.Errorf("get %s ticket: %w", ticketType, err)
		}
		return resp.Ticket, time.Duration(resp.ExpiresIn) * time.Second, nil
	}
}

// GetTicket 获取 ticket，优先读缓存。
func (c *Client) GetTicket(ctx context.Context, ticketType TicketType) (string, error) {
	ticket, ok := c.tickets[ticketType]
	if !ok {
		return "", fmt.Errorf("unsupported ticket type %q", ticketType)
	}
	return ticket.Get(ctx)
}

// RefreshTicket 跳过缓存，重新向微信获取 ticket。
func (c *Client) RefreshTicket(ctx context.Context, ticketType TicketType) (string, error) {
	ticket, ok := c.tickets[ticketType]
	if !ok {
		return "", fmt.Errorf("unsupported ticket type %q", ticketType)
	}
	return ticket.Refresh(ctx)
}
