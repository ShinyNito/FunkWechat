package officialaccount

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ShinyNito/FunkWechat/v3/wxcrypto"
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

	ticket, err := c.GetTicket(ctx, TicketTypeJSAPI)
	if err != nil {
		return JssdkSignResponse{}, fmt.Errorf("get jsapi_ticket: %w", err)
	}

	nonce := wxcrypto.RandomString(16)

	timestamp := time.Now().Unix()
	signature := wxcrypto.SignJSSDK(ticket, nonce, timestamp, trimmedURL)

	return JssdkSignResponse{
		AppID:     c.Config().AppID,
		Timestamp: timestamp,
		NonceStr:  nonce,
		Signature: signature,
	}, nil
}
