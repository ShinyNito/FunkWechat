package officialaccount

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

func TestOfficialAccount_GetTicket_NilRequest(t *testing.T) {
	oa, err := New(&Config{
		AppID:     "test_appid",
		AppSecret: "test_secret",
	})
	require.NoError(t, err)

	_, err = oa.GetTicket(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")

	err = oa.Get(context.Background(), "/cgi-bin/user/info", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid get result")

	err = oa.Post(context.Background(), "/cgi-bin/message/send", map[string]any{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid post result")
}
