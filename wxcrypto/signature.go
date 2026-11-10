// Package wxcrypto 微信相关的签名、验签和数据解密。
package wxcrypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// SHA1Sign 将参数按字典序排序后拼接，返回 SHA1 十六进制摘要。不修改传入的切片。
func SHA1Sign(params ...string) string {
	sum := sha1.Sum([]byte(joinSorted(params)))
	return hex.EncodeToString(sum[:])
}

// SHA256Sign 将参数按字典序排序后拼接，返回 SHA256 十六进制摘要。不修改传入的切片。
func SHA256Sign(params ...string) string {
	sum := sha256.Sum256([]byte(joinSorted(params)))
	return hex.EncodeToString(sum[:])
}

func joinSorted(params []string) string {
	sorted := slices.Clone(params)
	slices.Sort(sorted)
	return strings.Join(sorted, "")
}

// SignJSSDK 计算 JS-SDK 的 config 签名。url 不含 # 及其后的部分。
func SignJSSDK(ticket, nonce string, timestamp int64, url string) string {
	sum := sha1.Sum(fmt.Appendf(nil, "jsapi_ticket=%s&noncestr=%s&timestamp=%d&url=%s", ticket, nonce, timestamp, url))
	return hex.EncodeToString(sum[:])
}

// HMACSHA256 返回 HMAC-SHA256 十六进制摘要。
func HMACSHA256(data, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature 验证服务器配置和明文模式消息推送的签名。token 为开发者在后台配置的 Token。
func VerifySignature(signature, timestamp, nonce, token string) bool {
	computed := SHA1Sign(token, timestamp, nonce)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

// VerifyMsgSignature 验证安全模式消息推送的签名，encryptedMsg 为推送中的 Encrypt 字段。
func VerifyMsgSignature(msgSignature, timestamp, nonce, token, encryptedMsg string) bool {
	computed := SHA1Sign(token, timestamp, nonce, encryptedMsg)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(msgSignature)) == 1
}
