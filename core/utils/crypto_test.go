package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCS7PadAndUnpad(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		blockSize int
		wantLen   int
		wantErr   error
	}{
		{
			name:      "pad to block size",
			data:      []byte("hello"),
			blockSize: 8,
			wantLen:   8,
		},
		{
			name:      "already aligned",
			data:      []byte("12345678"),
			blockSize: 8,
			wantLen:   16,
		},
		{
			name:      "invalid block size on unpad",
			data:      []byte{1, 2, 3},
			blockSize: 4,
			wantErr:   ErrInvalidPKCS7Data,
		},
		{
			name:      "invalid padding value",
			data:      []byte{1, 2, 3, 0},
			blockSize: 4,
			wantErr:   ErrInvalidPKCS7Padding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := PKCS7Pad(tt.data, tt.blockSize)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantLen, len(padded))
			}

			if tt.wantErr != nil {
				_, err := PKCS7Unpad(tt.data, tt.blockSize)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			out, err := PKCS7Unpad(padded, tt.blockSize)
			require.NoError(t, err)
			assert.Equal(t, tt.data, out)
		})
	}
}

func TestAESCBCEncryptDecrypt(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	plaintext := []byte("secret message")

	ciphertext, err := AESCBCEncrypt(plaintext, key, iv)
	require.NoError(t, err)

	decrypted, err := AESCBCDecrypt(ciphertext, key, iv)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptUserData(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	payload := map[string]any{
		"nickName": "Alice",
		"gender":   1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	ciphertext, err := AESCBCEncrypt(raw, key, iv)
	require.NoError(t, err)

	sessionKey := base64.StdEncoding.EncodeToString(key)
	encryptedData := base64.StdEncoding.EncodeToString(ciphertext)
	ivStr := base64.StdEncoding.EncodeToString(iv)

	tests := []struct {
		name       string
		sessionKey string
		encrypted  string
		iv         string
		wantError  bool
		wantNick   string
		wantGender float64
	}{
		{
			name:       "valid decrypt to map",
			sessionKey: sessionKey,
			encrypted:  encryptedData,
			iv:         ivStr,
			wantNick:   "Alice",
			wantGender: 1,
		},
		{
			name:       "invalid base64",
			sessionKey: "###",
			encrypted:  encryptedData,
			iv:         ivStr,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := DecryptUserData(tt.sessionKey, tt.encrypted, tt.iv)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNick, data["nickName"])
			assert.Equal(t, tt.wantGender, data["gender"])
		})
	}
}

func TestAESCBCDecrypt_InvalidBlockSize(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	_, err := AESCBCDecrypt([]byte("short"), key, iv)
	assert.ErrorIs(t, err, ErrInvalidBlockSize)
}

func TestAESCBC_InvalidIVSize(t *testing.T) {
	key := []byte("1234567890abcdef")
	plaintext := []byte("secret")
	ciphertext := []byte("1234567890abcdef")
	invalidIV := []byte("short")

	_, err := AESCBCEncrypt(plaintext, key, invalidIV)
	assert.ErrorIs(t, err, ErrInvalidIVSize)

	_, err = AESCBCDecrypt(ciphertext, key, invalidIV)
	assert.ErrorIs(t, err, ErrInvalidIVSize)
}

func TestPKCS7Unpad_InvalidPaddingContent(t *testing.T) {
	data := []byte{1, 2, 3, 4, 2, 2, 2, 3}
	_, err := PKCS7Unpad(data, 8)
	assert.ErrorIs(t, err, ErrInvalidPKCS7Padding)

	data = []byte{1, 2, 3, 4, 9, 9, 9, 9}
	_, err = PKCS7Unpad(data, 8)
	assert.ErrorIs(t, err, ErrInvalidPKCS7Padding)

	// unchanged input on error
	assert.True(t, bytes.Equal([]byte{1, 2, 3, 4, 9, 9, 9, 9}, data))
}

func TestDecryptUserDataTo(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	payload := map[string]any{
		"nickName": "Bob",
		"gender":   2,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	ciphertext, err := AESCBCEncrypt(raw, key, iv)
	require.NoError(t, err)

	sessionKey := base64.StdEncoding.EncodeToString(key)
	encryptedData := base64.StdEncoding.EncodeToString(ciphertext)
	ivStr := base64.StdEncoding.EncodeToString(iv)

	var target struct {
		NickName string `json:"nickName"`
		Gender   int    `json:"gender"`
	}

	err = DecryptUserDataTo(sessionKey, encryptedData, ivStr, &target)
	require.NoError(t, err)
	assert.Equal(t, "Bob", target.NickName)
	assert.Equal(t, 2, target.Gender)
}

func TestRandomString(t *testing.T) {
	s, err := RandomString(16)
	require.NoError(t, err)
	assert.Len(t, s, 16)

	_, err = RandomString(-1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}
