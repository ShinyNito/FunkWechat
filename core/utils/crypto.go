package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomString 生成指定长度的随机字符串
func RandomString(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("length must be non-negative")
	}

	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			return "", fmt.Errorf("generate random int: %w", err)
		}
		b[i] = letterBytes[num.Int64()]
	}
	return string(b), nil
}

var (
	// ErrInvalidBlockSize 无效的块大小
	ErrInvalidBlockSize = errors.New("invalid block size")
	// ErrInvalidIVSize 无效的 IV 长度
	ErrInvalidIVSize = errors.New("invalid iv size")
	// ErrInvalidPKCS7Data 无效的 PKCS7 数据
	ErrInvalidPKCS7Data = errors.New("invalid PKCS7 data")
	// ErrInvalidPKCS7Padding 无效的 PKCS7 填充
	ErrInvalidPKCS7Padding = errors.New("invalid PKCS7 padding")
)

// DecryptUserData 解密微信用户敏感数据到目标类型。
// sessionKey: 用户会话密钥（Base64 编码）
// encryptedData: 加密数据（Base64 编码）
// iv: 初始向量（Base64 编码）
func DecryptUserData[T any](sessionKey, encryptedData, iv string) (T, error) {
	var zero T

	decrypted, err := decryptUserDataPlaintext(sessionKey, encryptedData, iv)
	if err != nil {
		return zero, err
	}

	var result T
	if err := json.Unmarshal(decrypted, &result); err != nil {
		return zero, fmt.Errorf("unmarshal json: %w", err)
	}
	return result, nil
}

func decryptUserDataPlaintext(sessionKey, encryptedData, iv string) ([]byte, error) {
	// Base64 解码
	keyBytes, err := base64.StdEncoding.DecodeString(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("decode session key: %w", err)
	}

	dataBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted data: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("decode iv: %w", err)
	}

	// AES-CBC 解密
	decrypted, err := AESCBCDecrypt(dataBytes, keyBytes, ivBytes)
	if err != nil {
		return nil, fmt.Errorf("aes decrypt: %w", err)
	}

	return decrypted, nil
}

// AESCBCDecrypt AES-CBC 解密
func AESCBCDecrypt(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	if len(iv) != aes.BlockSize {
		return nil, ErrInvalidIVSize
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidBlockSize
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidBlockSize
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS7 去填充
	plaintext, err = PKCS7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// AESCBCEncrypt AES-CBC 加密
func AESCBCEncrypt(plaintext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	if len(iv) != aes.BlockSize {
		return nil, ErrInvalidIVSize
	}

	// PKCS7 填充
	plaintext = PKCS7Pad(plaintext, aes.BlockSize)

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

// PKCS7Pad PKCS7 填充
func PKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// PKCS7Unpad PKCS7 去填充
func PKCS7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, ErrInvalidPKCS7Data
	}

	if length%blockSize != 0 {
		return nil, ErrInvalidPKCS7Data
	}

	padding := int(data[length-1])
	if padding > blockSize || padding == 0 {
		return nil, ErrInvalidPKCS7Padding
	}

	for i := range padding {
		if data[length-1-i] != byte(padding) {
			return nil, ErrInvalidPKCS7Padding
		}
	}

	return data[:length-padding], nil
}
