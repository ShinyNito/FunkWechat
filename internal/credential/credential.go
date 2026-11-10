// Package credential 管理 access_token、ticket 等带缓存的短期凭证。
package credential

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	// expiryBuffer 凭证提前过期的时间，避免临界时刻用到刚失效的凭证。
	expiryBuffer = 5 * time.Minute
	// fetchTimeout 单次获取的超时。获取不受调用方 ctx 取消的影响，需要独立的上限。
	fetchTimeout = 30 * time.Second
)

// Cache 凭证缓存，core.Cache 满足此接口。
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// FetchFunc 向微信获取凭证，返回凭证值和有效期。
type FetchFunc func(ctx context.Context) (value string, ttl time.Duration, err error)

// Credential 带缓存的短期凭证，例如 access_token、jsapi_ticket。
//
// 缓存未命中时，同一进程内的并发调用合并为一次获取。获取在后台进行，
// 不受发起者 ctx 取消的影响，一个调用方取消不会连累其他等待者。
type Credential struct {
	cache  Cache
	key    string
	fetch  FetchFunc
	logger *slog.Logger

	mu       sync.Mutex
	inflight *call
}

type call struct {
	done  chan struct{}
	value string
	err   error
}

func New(cache Cache, key string, fetch FetchFunc, logger *slog.Logger) *Credential {
	if logger == nil {
		logger = slog.Default()
	}
	return &Credential{cache: cache, key: key, fetch: fetch, logger: logger}
}

// Get 返回凭证，优先读缓存。
func (c *Credential) Get(ctx context.Context) (string, error) {
	if value, ok := c.cache.Get(ctx, c.key); ok {
		return value, nil
	}
	return c.wait(ctx, c.start(ctx, false))
}

// Refresh 跳过缓存重新获取凭证，用于凭证被微信拒绝后的重试。
// 已有获取在进行时直接等待它的结果。
func (c *Credential) Refresh(ctx context.Context) (string, error) {
	return c.wait(ctx, c.start(ctx, true))
}

func (c *Credential) start(ctx context.Context, force bool) *call {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight != nil {
		return c.inflight
	}
	pending := &call{done: make(chan struct{})}
	c.inflight = pending
	go c.run(context.WithoutCancel(ctx), pending, force)
	return pending
}

func (c *Credential) run(ctx context.Context, pending *call, force bool) {
	defer func() {
		c.mu.Lock()
		c.inflight = nil
		c.mu.Unlock()
		close(pending.done)
	}()

	// 等锁期间可能已有其他获取写入缓存，这是第二次也是最后一次查缓存。
	if !force {
		if value, ok := c.cache.Get(ctx, c.key); ok {
			pending.value = value
			return
		}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	value, ttl, err := c.fetch(fetchCtx)
	if err != nil {
		pending.err = err
		return
	}
	if value == "" {
		pending.err = errors.New("fetched empty credential")
		return
	}

	if err := c.cache.Set(ctx, c.key, value, cacheTTL(ttl)); err != nil {
		c.logger.LogAttrs(ctx, slog.LevelWarn, "cache wechat credential failed",
			slog.String("key", c.key), slog.Any("error", err))
	}
	pending.value = value
}

// cacheTTL 扣除提前过期时间。有效期过短时取一半，且至少 1 秒，避免写入永不过期的值。
func cacheTTL(ttl time.Duration) time.Duration {
	return max(ttl-expiryBuffer, ttl/2, time.Second)
}

func (c *Credential) wait(ctx context.Context, pending *call) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-pending.done:
		return pending.value, pending.err
	}
}
