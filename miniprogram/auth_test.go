package miniprogram

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

func TestMiniProgram_Code2Session(t *testing.T) {
	tests := []struct {
		name           string
		req            *Code2SessionRequest
		serverResponse map[string]any
		wantOpenID     string
		wantSessionKey string
		wantUnionID    string
		wantErrCode    int
		wantErr        bool
	}{
		{
			name: "成功获取 session（无 UnionID）",
			req: &Code2SessionRequest{
				JSCode: "081aBZ000X0pJt1WjY200zWDKK1aBZ0J",
			},
			serverResponse: map[string]any{
				"openid":      "test_openid",
				"session_key": "test_session_key",
			},
			wantOpenID:     "test_openid",
			wantSessionKey: "test_session_key",
		},
		{
			name: "成功获取 session（有 UnionID）",
			req: &Code2SessionRequest{
				JSCode: "081aBZ000X0pJt1WjY200zWDKK1aBZ0J",
			},
			serverResponse: map[string]any{
				"openid":      "test_openid",
				"session_key": "test_session_key",
				"unionid":     "test_unionid",
			},
			wantOpenID:     "test_openid",
			wantSessionKey: "test_session_key",
			wantUnionID:    "test_unionid",
		},
		{
			name: "微信 API 返回错误 - code 无效",
			req: &Code2SessionRequest{
				JSCode: "invalid_code",
			},
			serverResponse: map[string]any{
				"errcode": 40029,
				"errmsg":  "invalid code",
			},
			wantErr:     true,
			wantErrCode: 40029,
		},
		{
			name: "微信 API 返回错误 - code 被封禁",
			req: &Code2SessionRequest{
				JSCode: "banned_code",
			},
			serverResponse: map[string]any{
				"errcode": 40226,
				"errmsg":  "high risk user, code has been blocked",
			},
			wantErr:     true,
			wantErrCode: 40226,
		},
		{
			name: "微信 API 返回错误 - 系统繁忙",
			req: &Code2SessionRequest{
				JSCode: "test_code",
			},
			serverResponse: map[string]any{
				"errcode": -1,
				"errmsg":  "system error",
			},
			wantErr:     true,
			wantErrCode: -1,
		},
		{
			name: "JSCode 为空",
			req: &Code2SessionRequest{
				JSCode: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 如果 JSCode 为空，不需要启动服务器
			if tt.req.JSCode == "" {
				mp := New(&Config{
					AppID:     "test_appid",
					AppSecret: "test_secret",
				})

				_, err := mp.Code2Session(context.Background(), tt.req)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "js_code is required")
				return
			}

			// 创建测试服务器
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 验证请求方法
				assert.Equal(t, http.MethodGet, r.Method)

				// 验证路径
				assert.Equal(t, Code2SessionPath, r.URL.Path)

				// 验证查询参数
				query := r.URL.Query()
				assert.Equal(t, "test_appid", query.Get("appid"))
				assert.Equal(t, "test_secret", query.Get("secret"))
				assert.Equal(t, tt.req.JSCode, query.Get("js_code"))
				assert.Equal(t, "authorization_code", query.Get("grant_type"))

				// 验证没有 access_token（code2session 不需要）
				assert.Empty(t, query.Get("access_token"))

				// 返回响应
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			// 重定向请求到测试服务器
			targetURL, _ := url.Parse(server.URL)

			mp := New(&Config{
				AppID:     "test_appid",
				AppSecret: "test_secret",
				HTTPClient: &http.Client{
					Transport: &rewriteTransport{target: targetURL},
				},
			})

			// 执行测试
			resp, err := mp.Code2Session(context.Background(), tt.req)

			// 验证错误
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrCode != 0 {
					var we *core.WechatError
					require.ErrorAs(t, err, &we)
					assert.Equal(t, tt.wantErrCode, we.ErrCode)
				}
				return
			}

			// 验证成功响应
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.wantOpenID, resp.OpenID)
			assert.Equal(t, tt.wantSessionKey, resp.SessionKey)
			if tt.wantUnionID != "" {
				assert.Equal(t, tt.wantUnionID, resp.UnionID)
			}
		})
	}
}

func TestCode2SessionRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		jsCode  string
		wantErr bool
	}{
		{
			name:    "有效的 code",
			jsCode:  "081aBZ000X0pJt1WjY200zWDKK1aBZ0J",
			wantErr: false,
		},
		{
			name:    "空 code",
			jsCode:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := New(&Config{
				AppID:     "test_appid",
				AppSecret: "test_secret",
			})

			req := &Code2SessionRequest{
				JSCode: tt.jsCode,
			}

			_, err := mp.Code2Session(context.Background(), req)

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}
