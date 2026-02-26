package miniprogram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

const Code2SessionPath = "/sns/jscode2session"

type Code2SessionRequest struct {
	JSCode string
}

type Code2SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid,omitempty"`
}

type GetPhoneNumberRequest struct {
	Code string `json:"code"`
}

type GetPhoneNumberResponse struct {
	PhoneInfo PhoneInfo `json:"phone_info"`
}

type PhoneInfo struct {
	PhoneNumber     string      `json:"phoneNumber"`
	PurePhoneNumber string      `json:"purePhoneNumber"`
	CountryCode     CountryCode `json:"countryCode"`
	Watermark       Watermark   `json:"watermark"`
}

type CountryCode string

func (c *CountryCode) UnmarshalJSON(data []byte) error {
	if c == nil {
		return fmt.Errorf("countryCode target is nil")
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*c = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		*c = CountryCode(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(trimmed, &n); err == nil {
		*c = CountryCode(n.String())
		return nil
	}

	return fmt.Errorf("invalid countryCode: %s", string(trimmed))
}

type Watermark struct {
	AppID     string `json:"appid"`
	Timestamp int64  `json:"timestamp"`
}

func (c *Client) Code2Session(ctx context.Context, req Code2SessionRequest) (Code2SessionResponse, error) {
	if req.JSCode == "" {
		return Code2SessionResponse{}, fmt.Errorf("js_code is required")
	}

	return Request[Code2SessionResponse](c).
		Path(Code2SessionPath).
		Query("appid", c.cfg.AppID).
		Query("secret", c.cfg.AppSecret).
		Query("js_code", req.JSCode).
		Query("grant_type", "authorization_code").
		WithoutToken().
		Get(ctx)
}

func (c *Client) GetPhoneNumber(ctx context.Context, req GetPhoneNumberRequest) (GetPhoneNumberResponse, error) {
	if req.Code == "" {
		return GetPhoneNumberResponse{}, fmt.Errorf("code is required")
	}

	payload := map[string]string{"code": req.Code}
	return Request[GetPhoneNumberResponse](c).
		Path("/wxa/business/getuserphonenumber").
		Body(payload).
		Post(ctx)
}
