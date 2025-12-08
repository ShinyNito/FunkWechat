package officialaccount

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialAccount_GetJssdkSign(t *testing.T) {
	tests := []struct {
		name       string
		req        *JssdkSignRequest
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "成功生成签名",
			req: &JssdkSignRequest{
				URL: "https://example.com/path?query=1",
			},
			wantErr: false,
		},
		{
			name: "URL 包含 hash 时自动去除",
			req: &JssdkSignRequest{
				URL: "https://example.com/path#hash",
			},
			wantErr: false,
		},
		{
			name: "URL 为空报错",
			req: &JssdkSignRequest{
				URL: "",
			},
			wantErr:    true,
			wantErrMsg: "url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试服务器
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 返回 ticket 响应
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"errcode":    0,
					"errmsg":     "ok",
					"ticket":     "test_ticket",
					"expires_in": 7200,
				})
			}))
			defer server.Close()

			targetURL, _ := url.Parse(server.URL)
			cache := newStubCache()
			cache.Set(context.Background(), accessTokenCacheKeyPrefix+"test_appid", "test_access_token", 0)

			oa := New(&Config{
				AppID:     "test_appid",
				AppSecret: "test_secret",
				Cache:     cache,
				HTTPClient: &http.Client{
					Transport: &rewriteTransport{target: targetURL},
				},
			})

			resp, err := oa.GetJssdkSign(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)

			// 验证返回字段
			assert.Equal(t, "test_appid", resp.AppID)
			assert.NotEmpty(t, resp.Timestamp)
			assert.NotEmpty(t, resp.NonceStr)
			assert.Len(t, resp.NonceStr, 16)
			assert.NotEmpty(t, resp.Signature)
			assert.Len(t, resp.Signature, 40) // SHA1 生成的 hex 是 40 字符
		})
	}
}

func TestSign(t *testing.T) {
	// 使用官方文档的测试用例验证签名算法
	// 参考: https://developers.weixin.qq.com/doc/offiaccount/OA_Web_Apps/JS-SDK.html
	cache := newStubCache()
	oa := New(&Config{
		AppID:     "test_appid",
		AppSecret: "test_secret",
		Cache:     cache,
	})

	ticket := "sM4AOVdWfPE4DxkXGEs8VMCPGGVi4C3VM0P37wVUCFvkVAy_90u5h9nbSlYy3-Sl-HhTdfl2fzFy1AOcHKP7qg"
	nonceStr := "Wm3WZYTPz0wzccnW"
	timestamp := int64(1414587457)
	url := "http://mp.weixin.qq.com?params=value"

	signature := oa.sign(ticket, nonceStr, timestamp, url)

	// 手动计算预期签名
	signStr := fmt.Sprintf("jsapi_ticket=%s&noncestr=%s&timestamp=%d&url=%s",
		ticket, nonceStr, timestamp, url)
	h := sha1.New()
	h.Write([]byte(signStr))
	expected := hex.EncodeToString(h.Sum(nil))

	assert.Equal(t, expected, signature)
}
