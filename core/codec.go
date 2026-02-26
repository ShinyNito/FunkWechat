package core

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type wechatErrorEnvelope struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func DecodeWechat[T any](statusCode int, body []byte) (T, error) {
	var zero T

	if len(bytes.TrimSpace(body)) == 0 {
		if statusCode >= 200 && statusCode < 300 {
			return zero, nil
		}
		return zero, fmt.Errorf("http status %d", statusCode)
	}

	if wechatErr := parseWechatError(body); wechatErr != nil {
		return zero, wechatErr
	}

	if statusCode < 200 || statusCode >= 300 {
		return zero, fmt.Errorf("http status %d: %s", statusCode, truncateBody(body, 256))
	}

	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

func parseWechatError(body []byte) error {
	var envelope wechatErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	if envelope.ErrCode != 0 {
		return NewWechatError(envelope.ErrCode, envelope.ErrMsg)
	}
	return nil
}

func truncateBody(body []byte, max int) string {
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}
