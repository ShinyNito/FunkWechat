package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSHA1Sign(t *testing.T) {
	tests := []struct {
		name   string
		params []string
		want   string
	}{
		{
			name:   "sorted automatically",
			params: []string{"nonce", "token", "timestamp"},
			want:   "6db4861c77e0633e0105672fcd41c9fc2766e26e",
		},
		{
			name:   "single param",
			params: []string{"abc"},
			want:   "a9993e364706816aba3e25717850c26c9cd0d89d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA1Sign(tt.params...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSHA256Sign(t *testing.T) {
	got := SHA256Sign("c", "a", "b")
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", got)
}

func TestHMACSHA256(t *testing.T) {
	got := HMACSHA256("hello", "key")
	assert.Equal(t, "9307b3b915efb5171ff14d8cb55fbcc798c6c0ef1456d66ded1a6aa723a58b7b", got)
}

func TestVerifySignature(t *testing.T) {
	assert.True(t, VerifySignature("6db4861c77e0633e0105672fcd41c9fc2766e26e", "timestamp", "nonce", "token"))
	assert.False(t, VerifySignature("invalid", "timestamp", "nonce", "token"))
}

func TestVerifyMsgSignature(t *testing.T) {
	assert.True(t, VerifyMsgSignature("aa7fc06a800892bacf85c0ce5a37f057dbe560ca", "ts", "nonce", "token", "encrypted"))
	assert.False(t, VerifyMsgSignature("invalid", "ts", "nonce", "token", "encrypted"))
}
