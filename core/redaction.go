package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const redactedValue = "***"

// sensitiveKeys 日志和错误信息中需要脱敏的字段，键为小写。包括凭证和个人信息。
var sensitiveKeys = map[string]struct{}{
	// 凭证
	"access_token":  {},
	"appsecret":     {},
	"app_secret":    {},
	"authorization": {},
	"client_secret": {},
	"code":          {},
	"js_code":       {},
	"refresh_token": {},
	"secret":        {},
	"session_key":   {},
	"ticket":        {},
	"token":         {},
	// 个人信息
	"encrypteddata":   {},
	"iv":              {},
	"openid":          {},
	"phonenumber":     {},
	"purephonenumber": {},
	"touser":          {},
	"unionid":         {},
}

// redactURL 脱敏 URL 查询参数中的敏感字段。无法解析时丢弃整个查询串，不原样返回。
func redactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		base, _, _ := strings.Cut(rawURL, "?")
		return base
	}
	if parsed.RawQuery == "" {
		return rawURL
	}

	query := parsed.Query()
	for key, values := range query {
		if !isSensitiveKey(key) {
			continue
		}
		for i := range values {
			values[i] = redactedValue
		}
		query[key] = values
	}

	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// redactURLError 脱敏 *url.Error 中的 URL，保留错误链，超时等判断不受影响。
func redactURLError(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		urlErr.URL = redactURL(urlErr.URL)
	}
	return err
}

// redactBody 脱敏 JSON 请求体或响应体中的敏感字段，用于日志输出。
// 嵌套对象和数组中的同名字段也会被脱敏；multipart、图片等非 JSON 内容只输出长度。
func redactBody(body []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Sprintf("<non-JSON body, %d bytes>", len(body))
	}
	redacted, err := json.Marshal(redactJSONValue(value))
	if err != nil {
		return fmt.Sprintf("<unencodable body, %d bytes>", len(body))
	}
	return string(redacted)
}

func redactJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if isSensitiveKey(key) {
				v[key] = redactedValue
				continue
			}
			v[key] = redactJSONValue(item)
		}
	case []any:
		for i, item := range v {
			v[i] = redactJSONValue(item)
		}
	}
	return value
}

func isSensitiveKey(key string) bool {
	_, exists := sensitiveKeys[strings.ToLower(key)]
	return exists
}
