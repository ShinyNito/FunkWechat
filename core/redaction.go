package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const redactedValue = "***"

var sensitiveKeys = map[string]struct{}{
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
	"token":         {},
}

// RedactQueryMap 脱敏查询参数，返回拷贝，原 map 不会被修改。
func RedactQueryMap(query map[string]string) map[string]string {
	if query == nil {
		return nil
	}

	out := make(map[string]string, len(query))
	for key, value := range query {
		if isSensitiveKey(key) {
			out[key] = redactedValue
			continue
		}
		out[key] = value
	}

	return out
}

// RedactURLQuery 脱敏 URL 查询参数中的敏感字段。
func RedactURLQuery(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
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

// RedactJSONBody 脱敏 JSON 请求体或响应体中的敏感字段，用于日志输出。
// 嵌套对象和数组中的同名字段也会被脱敏；无法解析为 JSON 的内容只输出长度。
func RedactJSONBody(body []byte) string {
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
