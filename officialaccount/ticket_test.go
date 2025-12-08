package officialaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ShinyNito/FunkWechat/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialAccount_GetTicket(t *testing.T) {
	tests := []struct {
		name           string
		req            *GetTicketRequest
		cacheValue     string
		serverResponse map[string]any
		wantTicket     string
		wantErrCode    int
		wantErr        bool
	}{
		{
			name: "成功获取 jsapi ticket",
			req: &GetTicketRequest{
				Type: TicketTypeJSAPI,
			},
			serverResponse: map[string]any{
				"errcode":    0,
				"errmsg":     "ok",
				"ticket":     "test_jsapi_ticket",
				"expires_in": 7200,
			},
			wantTicket: "test_jsapi_ticket",
		},
		{
			name: "从缓存获取 ticket",
			req: &GetTicketRequest{
				Type: TicketTypeJSAPI,
			},
			cacheValue: "cached_ticket",
			wantTicket: "cached_ticket",
		},
		{
			name: "默认类型为 jsapi",
			req:  &GetTicketRequest{},
			serverResponse: map[string]any{
				"errcode":    0,
				"errmsg":     "ok",
				"ticket":     "default_jsapi_ticket",
				"expires_in": 7200,
			},
			wantTicket: "default_jsapi_ticket",
		},
		{
			name: "获取 wx_card ticket",
			req: &GetTicketRequest{
				Type: TicketTypeWxCard,
			},
			serverResponse: map[string]any{
				"errcode":    0,
				"errmsg":     "ok",
				"ticket":     "test_wxcard_ticket",
				"expires_in": 7200,
			},
			wantTicket: "test_wxcard_ticket",
		},
		{
			name: "微信 API 返回错误",
			req: &GetTicketRequest{
				Type: TicketTypeJSAPI,
			},
			serverResponse: map[string]any{
				"errcode": 40001,
				"errmsg":  "invalid credential",
			},
			wantErr:     true,
			wantErrCode: 40001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试服务器
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 验证请求
				assert.Equal(t, http.MethodGet, r.Method)

				// 返回响应
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			targetURL, _ := url.Parse(server.URL)

			cache := newStubCache()

			// 设置 access_token 缓存（GetTicket 需要 access_token）
			cache.Set(context.Background(), accessTokenCacheKeyPrefix+"test_appid", "test_access_token", 0)

			// 设置 ticket 缓存
			if tt.cacheValue != "" {
				ticketType := tt.req.Type
				if ticketType == "" {
					ticketType = TicketTypeJSAPI
				}
				cacheKey := jsapiTicketCacheKeyPrefix + "test_appid:" + string(ticketType)
				cache.Set(context.Background(), cacheKey, tt.cacheValue, 0)
			}

			oa := New(&Config{
				AppID:     "test_appid",
				AppSecret: "test_secret",
				Cache:     cache,
				HTTPClient: &http.Client{
					Transport: &rewriteTransport{target: targetURL},
				},
			})

			resp, err := oa.GetTicket(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrCode != 0 {
					var we *core.WechatError
					require.ErrorAs(t, err, &we)
					assert.Equal(t, tt.wantErrCode, we.ErrCode)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.wantTicket, resp.Ticket)
		})
	}
}
