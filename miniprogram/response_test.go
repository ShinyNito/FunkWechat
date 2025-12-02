package miniprogram

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ShinyNito/FunkWechat/core"
	"github.com/stretchr/testify/assert"
)

func TestResponse_Error(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantErr error
	}{
		{
			name:    "success",
			body:    []byte(`{"errcode":0,"errmsg":"ok"}`),
			wantErr: nil,
		},
		{
			name:    "wechat error",
			body:    []byte(`{"errcode":40001,"errmsg":"invalid credential"}`),
			wantErr: core.NewWechatError(40001, "invalid credential"),
		},
		{
			name:    "invalid json",
			body:    []byte("not-json"),
			wantErr: core.NewResponseParseError([]byte("not-json"), errors.New("invalid character")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewResponse(tt.body)
			err := resp.Error()
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}

			switch expected := tt.wantErr.(type) {
			case *core.WechatError:
				var we *core.WechatError
				assert.ErrorAs(t, err, &we)
				assert.Equal(t, expected.ErrCode, we.ErrCode)
				assert.Equal(t, expected.ErrMsg, we.ErrMsg)
			case *core.ResponseParseError:
				var pe *core.ResponseParseError
				assert.ErrorAs(t, err, &pe)
				assert.Equal(t, tt.body, pe.Body)
			default:
				t.Fatalf("unsupported expected error type %T", expected)
			}
		})
	}
}

func TestResponse_JSONAndMap(t *testing.T) {
	type payload struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}

	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "valid json",
			body: []byte(`{"foo":"bar","bar":123}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewResponse(tt.body)

			var p payload
			assert.NoError(t, resp.JSON(&p))
			assert.Equal(t, "bar", p.Foo)
			assert.Equal(t, 123, p.Bar)

			m, err := resp.Map()
			assert.NoError(t, err)
			assert.Equal(t, "bar", m["foo"])
			assert.Equal(t, float64(123), m["bar"])
		})
	}
}

func TestResponse_String(t *testing.T) {
	data := map[string]string{"msg": "hello"}
	raw, _ := json.Marshal(data)

	resp := NewResponse(raw)
	assert.Equal(t, string(raw), resp.String())
}
