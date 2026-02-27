package utils

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"strings"
)

// SHA1Sign 使用 SHA1 计算签名
// 将参数按字典序排序后拼接，再计算 SHA1 哈希
func SHA1Sign(params ...string) string {
	sort.Strings(params)
	str := strings.Join(params, "")
	h := sha1.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256Sign 使用 SHA256 计算签名
func SHA256Sign(params ...string) string {
	sort.Strings(params)
	str := strings.Join(params, "")
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// HMACSHA256 使用 HMAC-SHA256 计算签名
func HMACSHA256(data, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature 验证微信服务器签名
// signature: 微信传来的签名
// timestamp: 时间戳
// nonce: 随机字符串
// token: 开发者配置的 Token
func VerifySignature(signature, timestamp, nonce, token string) bool {
	computed := SHA1Sign(token, timestamp, nonce)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

// VerifyMsgSignature 验证消息签名（加密消息模式）
// msgSignature: 消息签名
// timestamp: 时间戳
// nonce: 随机字符串
// token: 开发者配置的 Token
// encryptedMsg: 加密的消息体
func VerifyMsgSignature(msgSignature, timestamp, nonce, token, encryptedMsg string) bool {
	computed := SHA1Sign(token, timestamp, nonce, encryptedMsg)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(msgSignature)) == 1
}
