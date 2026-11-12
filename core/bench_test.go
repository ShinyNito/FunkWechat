package core

import "testing"

type benchMessageValue struct {
	Value string `json:"value"`
}

type benchMessageRequest struct {
	ToUser           string                       `json:"touser"`
	TemplateID       string                       `json:"template_id"`
	Page             string                       `json:"page,omitempty"`
	Data             map[string]benchMessageValue `json:"data"`
	MiniProgramState string                       `json:"miniprogram_state,omitempty"`
}

func messageRequestBody() benchMessageRequest {
	return benchMessageRequest{
		ToUser: "openid", TemplateID: "template", Page: "/pages/lottery-detail/index?int=7", MiniProgramState: "trial",
		Data: map[string]benchMessageValue{
			"thing6": {Value: "双十一限定公仔抽奖活动..."},
			"thing2": {Value: "公仔"},
			"thing4": {Value: "已中奖"},
			"time7":  {Value: "2026-11-12 20:00:00"},
			"thing1": {Value: "可点击前往小程序查看"},
		},
	}
}

func BenchmarkEncodeRequest(b *testing.B) {
	r := &Request{jsonBody: messageRequestBody(), hasJSONBody: true}
	b.ReportAllocs()
	for b.Loop() {
		data, _, err := r.encode()
		if err != nil || len(data) == 0 {
			b.Fatalf("encode returned %d bytes, error %v", len(data), err)
		}
	}
}

type benchResp struct {
	Status
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
}

func BenchmarkDecodeResponse(b *testing.B) {
	resp := response{status: 200, body: []byte(`{"errcode":0,"errmsg":"ok","openid":"oABCD1234567890","session_key":"c2Vzc2lvbmtleQ==","unionid":"u1234567890"}`)}
	for b.Loop() {
		var out benchResp
		if err := decodeResponse("/x", resp, &out); err != nil || out.OpenID == "" {
			b.Fatal(err)
		}
	}
}
