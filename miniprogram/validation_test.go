package miniprogram

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ValidateConfig(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")

	_, err = New(&Config{AppSecret: "secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appid is required")

	_, err = New(&Config{AppID: "appid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appsecret is required")
}

func TestMiniProgram_NilRequestValidation(t *testing.T) {
	mp, err := New(&Config{
		AppID:     "test_appid",
		AppSecret: "test_secret",
	})
	require.NoError(t, err)

	_, err = mp.Code2Session(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")

	_, err = mp.GetPhoneNumber(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")

	err = mp.Get(context.Background(), "/cgi-bin/user/info", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid get result")

	err = mp.Post(context.Background(), "/cgi-bin/message/send", map[string]any{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid post result")
}
