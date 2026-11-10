// Package credential 管理 access_token、ticket 等带缓存的短期凭证。
package credential

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// expiryBuffer 凭证提前过期的时间，避免临界时刻用到刚失效的凭证。
	expiryBuffer = 5 * time.Minute
	// fetchTimeout 单次获取的超时。获取不受调用方 ctx 取消的影响，需要独立的上限。
	fetchTimeout = 30 * time.Second
	// storeRecheck 从外部存储读到的值在进程内保留的时间。外部存储不返回剩余有效期，
	// 所以到期后重新读取。必须小于 expiryBuffer，保证这段时间内凭证不会真正过期。
	storeRecheck = time.Minute
)

// Store 可选的外部凭证存储，用于多个进程共享凭证，core.Cache 满足此接口。
type Store interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// FetchFunc 向微信获取凭证，返回凭证值和有效期。
type FetchFunc func(ctx context.Context) (value string, ttl time.Duration, err error)

// Credential 带缓存的短期凭证，例如 access_token、jsapi_ticket。
//
// 凭证保存在进程内存里，未过期时 Get 只做一次原子读。过期后先查外部存储（如果有），
// 再向微信获取。同一进程内的并发获取合并为一次，获取在后台进行，
// 不受发起者 ctx 取消的影响，一个调用方取消不会连累其他等待者。
type Credential struct {
	store  Store // 可为 nil
	key    string
	fetch  FetchFunc
	logger *slog.Logger

	current atomic.Pointer[entry]

	mu       sync.Mutex
	inflight *call
}

type entry struct {
	value     string
	expiresAt time.Time
}

type call struct {
	done   chan struct{}
	forced bool
	value  string
	err    error
}

// New 创建凭证。store 为 nil 时只使用进程内存；key 是外部存储的键。
func New(store Store, key string, fetch FetchFunc, logger *slog.Logger) *Credential {
	if logger == nil {
		logger = slog.Default()
	}
	return &Credential{store: store, key: key, fetch: fetch, logger: logger}
}

// Get 返回凭证，优先使用内存中未过期的值。
func (c *Credential) Get(ctx context.Context) (string, error) {
	if value, ok := c.cached(); ok {
		return value, nil
	}
	return c.wait(ctx, c.start(ctx, false))
}

// Refresh 跳过内存和外部存储，重新向微信获取凭证，用于凭证被微信拒绝后的重试。
func (c *Credential) Refresh(ctx context.Context) (string, error) {
	for {
		pending := c.start(ctx, true)
		value, err := c.wait(ctx, pending)
		// 加入的是一次普通获取时，它的结果可能来自外部存储里的旧值，需要再发起一次强制获取。
		if pending.forced || ctx.Err() != nil {
			return value, err
		}
	}
}

func (c *Credential) cached() (string, bool) {
	e := c.current.Load()
	if e == nil || !time.Now().Before(e.expiresAt) {
		return "", false
	}
	return e.value, true
}

func (c *Credential) start(ctx context.Context, force bool) *call {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight != nil {
		return c.inflight
	}
	pending := &call{done: make(chan struct{}), forced: force}
	c.inflight = pending
	go c.run(context.WithoutCancel(ctx), pending)
	return pending
}

func (c *Credential) run(ctx context.Context, pending *call) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	pending.value, pending.err = c.load(ctx, pending.forced)

	c.mu.Lock()
	c.inflight = nil
	c.mu.Unlock()
	close(pending.done)
}

func (c *Credential) load(ctx context.Context, force bool) (string, error) {
	if !force {
		// 排队期间可能已有其他获取完成。
		if value, ok := c.cached(); ok {
			return value, nil
		}
		if value, ok := c.loadFromStore(ctx); ok {
			return value, nil
		}
	}

	value, ttl, err := c.fetch(ctx)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New("fetched empty credential")
	}

	ttl = usableTTL(ttl)
	c.current.Store(&entry{value: value, expiresAt: time.Now().Add(ttl)})
	if c.store != nil {
		if err := c.store.Set(ctx, c.key, value, ttl); err != nil {
			c.logger.LogAttrs(ctx, slog.LevelWarn, "store wechat credential failed",
				slog.String("key", c.key), slog.Any("error", err))
		}
	}
	return value, nil
}

// loadFromStore 外部存储出错时记录日志并当作未命中，由调用方改为向微信获取。
func (c *Credential) loadFromStore(ctx context.Context) (string, bool) {
	if c.store == nil {
		return "", false
	}
	value, ok, err := c.store.Get(ctx, c.key)
	if err != nil {
		c.logger.LogAttrs(ctx, slog.LevelWarn, "load wechat credential from store failed",
			slog.String("key", c.key), slog.Any("error", err))
		return "", false
	}
	if !ok || value == "" {
		return "", false
	}
	c.current.Store(&entry{value: value, expiresAt: time.Now().Add(storeRecheck)})
	return value, true
}

// usableTTL 扣除提前过期时间。有效期过短时取一半，且至少 1 秒，避免写入永不过期的值。
func usableTTL(ttl time.Duration) time.Duration {
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
