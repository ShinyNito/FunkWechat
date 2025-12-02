package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWechatError_Error(t *testing.T) {
	tests := []struct {
		name    string
		errCode int
		errMsg  string
		want    string
	}{
		{
			name:    "normal error",
			errCode: 40001,
			errMsg:  "invalid credential",
			want:    "wechat error: [40001] invalid credential",
		},
		{
			name:    "success code",
			errCode: 0,
			errMsg:  "ok",
			want:    "wechat error: [0] ok",
		},
		{
			name:    "system busy",
			errCode: -1,
			errMsg:  "system error",
			want:    "wechat error: [-1] system error",
		},
		{
			name:    "empty message",
			errCode: 40013,
			errMsg:  "",
			want:    "wechat error: [40013] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewWechatError(tt.errCode, tt.errMsg)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestWechatError_IsSuccess(t *testing.T) {
	tests := []struct {
		name    string
		errCode int
		want    bool
	}{
		{
			name:    "success",
			errCode: 0,
			want:    true,
		},
		{
			name:    "error code",
			errCode: 40001,
			want:    false,
		},
		{
			name:    "system busy",
			errCode: -1,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewWechatError(tt.errCode, "test")
			assert.Equal(t, tt.want, err.IsSuccess())
		})
	}
}

func TestIsTokenError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "invalid token error",
			err:  NewWechatError(ErrCodeInvalidToken, "invalid token"),
			want: true,
		},
		{
			name: "expired token error",
			err:  NewWechatError(ErrCodeExpiredToken, "expired token"),
			want: true,
		},
		{
			name: "other wechat error",
			err:  NewWechatError(ErrCodeInvalidAppID, "invalid appid"),
			want: false,
		},
		{
			name: "success code",
			err:  NewWechatError(ErrCodeSuccess, "ok"),
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTokenError(tt.err))
		})
	}
}

func TestWechatError_Fields(t *testing.T) {
	err := &WechatError{
		ErrCode: 40029,
		ErrMsg:  "invalid code",
	}

	assert.Equal(t, 40029, err.ErrCode)
	assert.Equal(t, "invalid code", err.ErrMsg)
}

func TestErrorCodeConstants(t *testing.T) {
	assert.Equal(t, 0, ErrCodeSuccess)
	assert.Equal(t, -1, ErrCodeBusy)
	assert.Equal(t, 40001, ErrCodeInvalidToken)
	assert.Equal(t, 42001, ErrCodeExpiredToken)
	assert.Equal(t, 40013, ErrCodeInvalidAppID)
	assert.Equal(t, 40125, ErrCodeInvalidAppSecret)
	assert.Equal(t, 40029, ErrCodeInvalidCode)
	assert.Equal(t, 40163, ErrCodeCodeUsed)
	assert.Equal(t, 45011, ErrCodeFreqLimit)
	assert.Equal(t, 48001, ErrCodeAPIUnauthorized)
}

func TestResponseParseError(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wrapped error
	}{
		{
			name:    "json unmarshal error",
			body:    []byte("not-json"),
			wrapped: errors.New("invalid character"),
		},
		{
			name:    "empty body",
			body:    []byte{},
			wrapped: errors.New("EOF"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewResponseParseError(tt.body, tt.wrapped)
			assert.Equal(t, tt.body, err.Body)
			assert.ErrorIs(t, err, tt.wrapped)
			assert.Contains(t, err.Error(), "failed to parse response")
		})
	}
}
