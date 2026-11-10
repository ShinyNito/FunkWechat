package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestWechatError(t *testing.T) {
	err := &WechatError{Path: "/cgi-bin/x", ErrCode: 40001, ErrMsg: "invalid"}
	if err.Error() != "wechat /cgi-bin/x: [40001] invalid" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestWechatErrorIsMatchesByCode(t *testing.T) {
	err := fmt.Errorf("send: %w", &WechatError{Path: "/x", ErrCode: 43101, ErrMsg: "refused"})
	if !errors.Is(err, ErrUserRefused) {
		t.Fatal("expected errors.Is to match ErrUserRefused")
	}
	if errors.Is(err, ErrInvalidCode) {
		t.Fatal("unexpected match with a different errcode")
	}
	if errors.Is(errors.New("plain"), ErrUserRefused) {
		t.Fatal("plain error must not match")
	}
}

func TestIsTokenError(t *testing.T) {
	for _, code := range []int{40001, 40014, 42001} {
		if !IsTokenError(fmt.Errorf("wrapped: %w", &WechatError{ErrCode: code})) {
			t.Errorf("errcode %d should be a token error", code)
		}
	}
	if IsTokenError(&WechatError{ErrCode: ErrInvalidAppID.ErrCode}) {
		t.Fatal("unexpected token error")
	}
	if IsTokenError(errors.New("plain")) || IsTokenError(nil) {
		t.Fatal("unexpected token error for plain or nil error")
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"busy", &WechatError{ErrCode: -1}, true},
		{"business error", &WechatError{ErrCode: 40029}, false},
		{"http 502", &HTTPError{StatusCode: 502}, true},
		{"http 429", &HTTPError{StatusCode: 429}, true},
		{"http 404", &HTTPError{StatusCode: 404}, false},
		{"network timeout", &url.Error{Op: "Get", URL: "http://x", Err: timeoutError{}}, true},
		{"internal deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), true},
		{"canceled", fmt.Errorf("x: %w", context.Canceled), false},
		{"invalid argument", fmt.Errorf("%w: code is required", ErrInvalidArgument), false},
	}
	for _, tt := range tests {
		if got := IsRetryable(context.Background(), tt.err); got != tt.want {
			t.Errorf("%s: IsRetryable = %v; want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsRetryableFalseWhenCallerContextDone(t *testing.T) {
	timeout := &url.Error{Op: "Get", URL: "http://x", Err: context.DeadlineExceeded}

	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if IsRetryable(expired, timeout) {
		t.Error("expired caller context should not be retryable")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if IsRetryable(canceled, &HTTPError{StatusCode: 503}) {
		t.Error("canceled caller context should not be retryable")
	}
}
