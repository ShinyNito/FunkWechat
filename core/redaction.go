package core

import (
	"net/url"
	"strings"
)

const redactedValue = "***"

var sensitiveQueryKeys = map[string]struct{}{
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
		if isSensitiveQueryKey(key) {
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
		if !isSensitiveQueryKey(key) {
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

func isSensitiveQueryKey(key string) bool {
	_, exists := sensitiveQueryKeys[strings.ToLower(key)]
	return exists
}
