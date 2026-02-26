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

type JssdkSignRequest struct {
	URL string
}

type JssdkSignResponse struct {
	AppID     string `json:"appId"`
	Timestamp int64  `json:"timestamp"`
	NonceStr  string `json:"nonceStr"`
	Signature string `json:"signature"`
}

func (c *Client) GetJssdkSign(ctx context.Context, req JssdkSignRequest) (JssdkSignResponse, error) {
	if req.URL == "" {
		return JssdkSignResponse{}, fmt.Errorf("url is required")
	}

	trimmedURL := req.URL
	if idx := strings.Index(trimmedURL, "#"); idx >= 0 {
		trimmedURL = trimmedURL[:idx]
	}

	ticketResp, err := c.GetTicket(ctx, GetTicketRequest{Type: TicketTypeJSAPI})
	if err != nil {
		return JssdkSignResponse{}, fmt.Errorf("get jsapi_ticket: %w", err)
	}

	nonce, err := utils.RandomString(16)
	if err != nil {
		return JssdkSignResponse{}, fmt.Errorf("generate nonce: %w", err)
	}

	timestamp := time.Now().Unix()
	signature := c.sign(ticketResp.Ticket, nonce, timestamp, trimmedURL)

	return JssdkSignResponse{
		AppID:     c.cfg.AppID,
		Timestamp: timestamp,
		NonceStr:  nonce,
		Signature: signature,
	}, nil
}

func (c *Client) sign(ticket, nonce string, timestamp int64, rawURL string) string {
	signStr := fmt.Sprintf("jsapi_ticket=%s&noncestr=%s&timestamp=%d&url=%s", ticket, nonce, timestamp, rawURL)

	h := sha1.New()
	_, _ = h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}
