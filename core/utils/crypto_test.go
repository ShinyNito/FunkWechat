package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
				if len(padded) != tt.wantLen {
					t.Errorf("got %v; want %v", len(padded), tt.wantLen)
				}
			}

			if tt.wantErr != nil {
				_, err := PKCS7Unpad(tt.data, tt.blockSize)
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v; want %v", err, tt.wantErr)
				}
				return
			}

			out, err := PKCS7Unpad(padded, tt.blockSize)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(out, tt.data) {
				t.Errorf("got %q; want %q", out, tt.data)
			}
		})
	}
}

func TestAESCBCEncryptDecrypt(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	plaintext := []byte("secret message")

	ciphertext, err := AESCBCEncrypt(plaintext, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decrypted, err := AESCBCDecrypt(ciphertext, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("got %q; want %q", decrypted, plaintext)
	}
}

func TestDecryptUserData(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	type userData struct {
		NickName string `json:"nickName"`
		Gender   int    `json:"gender"`
	}
	payload := userData{NickName: "Alice", Gender: 1}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ciphertext, err := AESCBCEncrypt(raw, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
		wantGender int
	}{
		{
			name:       "valid decrypt",
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
			data, err := DecryptUserData[userData](tt.sessionKey, tt.encrypted, tt.iv)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if data.NickName != tt.wantNick {
				t.Errorf("got %v; want %v", data.NickName, tt.wantNick)
			}
			if data.Gender != tt.wantGender {
				t.Errorf("got %v; want %v", data.Gender, tt.wantGender)
			}
		})
	}
}

func TestAESCBCDecrypt_InvalidBlockSize(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	_, err := AESCBCDecrypt([]byte("short"), key, iv)
	if !errors.Is(err, ErrInvalidBlockSize) {
		t.Errorf("error = %v; want %v", err, ErrInvalidBlockSize)
	}
}

func TestAESCBC_InvalidIVSize(t *testing.T) {
	key := []byte("1234567890abcdef")
	plaintext := []byte("secret")
	ciphertext := []byte("1234567890abcdef")
	invalidIV := []byte("short")

	_, err := AESCBCEncrypt(plaintext, key, invalidIV)
	if !errors.Is(err, ErrInvalidIVSize) {
		t.Errorf("error = %v; want %v", err, ErrInvalidIVSize)
	}

	_, err = AESCBCDecrypt(ciphertext, key, invalidIV)
	if !errors.Is(err, ErrInvalidIVSize) {
		t.Errorf("error = %v; want %v", err, ErrInvalidIVSize)
	}
}

func TestPKCS7Unpad_InvalidPaddingContent(t *testing.T) {
	data := []byte{1, 2, 3, 4, 2, 2, 2, 3}
	_, err := PKCS7Unpad(data, 8)
	if !errors.Is(err, ErrInvalidPKCS7Padding) {
		t.Errorf("error = %v; want %v", err, ErrInvalidPKCS7Padding)
	}

	data = []byte{1, 2, 3, 4, 9, 9, 9, 9}
	_, err = PKCS7Unpad(data, 8)
	if !errors.Is(err, ErrInvalidPKCS7Padding) {
		t.Errorf("error = %v; want %v", err, ErrInvalidPKCS7Padding)
	}

	// unchanged input on error
	if !(bytes.Equal([]byte{1, 2, 3, 4, 9, 9, 9, 9}, data)) {
		t.Errorf("expected true")
	}
}

func TestDecryptUserDataWithDifferentStruct(t *testing.T) {
	key := []byte("1234567890abcdef")
	iv := []byte("abcdef1234567890")
	payload := struct {
		NickName string `json:"nickName"`
		Gender   int    `json:"gender"`
	}{NickName: "Bob", Gender: 2}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ciphertext, err := AESCBCEncrypt(raw, key, iv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessionKey := base64.StdEncoding.EncodeToString(key)
	encryptedData := base64.StdEncoding.EncodeToString(ciphertext)
	ivStr := base64.StdEncoding.EncodeToString(iv)

	target, err := DecryptUserData[struct {
		NickName string `json:"nickName"`
		Gender   int    `json:"gender"`
	}](sessionKey, encryptedData, ivStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.NickName != "Bob" {
		t.Errorf("got %v; want %v", target.NickName, "Bob")
	}
	if target.Gender != 2 {
		t.Errorf("got %v; want %v", target.Gender, 2)
	}
}

func TestRandomString(t *testing.T) {
	s, err := RandomString(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 16 {
		t.Errorf("len = %d; want 16", len(s))
	}

	_, err = RandomString(-1)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("%q does not contain %q", err.Error(), "non-negative")
	}
}
