package miniprogram

import (
	"context"
	"fmt"
)

const SendSubscribeMessagePath = "/cgi-bin/message/subscribe/send"

// MiniProgramState 跳转小程序的版本。
type MiniProgramState string

const (
	MiniProgramStateFormal    MiniProgramState = "formal"
	MiniProgramStateTrial     MiniProgramState = "trial"
	MiniProgramStateDeveloper MiniProgramState = "developer"
)

// SubscribeMessageRequest 订阅消息内容。
// Data 的键为模板字段名（如 thing1、time7），值的长度和格式需符合模板字段类型的限制。
type SubscribeMessageRequest struct {
	ToUser           string                           `json:"touser"`
	TemplateID       string                           `json:"template_id"`
	Page             string                           `json:"page,omitempty"`
	Data             map[string]SubscribeMessageValue `json:"data"`
	MiniProgramState MiniProgramState                 `json:"miniprogram_state,omitempty"`
	Lang             string                           `json:"lang,omitempty"`
}

type SubscribeMessageValue struct {
	Value string `json:"value"`
}

// SendSubscribeMessage 发送订阅消息。
// 用户未订阅或订阅次数已用完时返回 errcode 为 core.ErrCodeUserRefused 的 *core.WechatError。
func (c *Client) SendSubscribeMessage(ctx context.Context, req SubscribeMessageRequest) error {
	if req.ToUser == "" {
		return fmt.Errorf("touser is required")
	}
	if req.TemplateID == "" {
		return fmt.Errorf("template_id is required")
	}

	_, err := Request[struct{}](c).
		Path(SendSubscribeMessagePath).
		Body(req).
		Post(ctx)
	return err
}
