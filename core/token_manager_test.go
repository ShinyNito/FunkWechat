package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tokenTestCache struct {
	mu   sync.RWMutex
	data map[string]string
}

func newTokenTestCache() *tokenTestCache {
	return &tokenTestCache{data: make(map[string]string)}
}

func (c *tokenTestCache) Get(_ context.Context, key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *tokenTestCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *tokenTestCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func TestTokenManagerSingleflight(t *testing.T) {
	cache := newTokenTestCache()
	var calls int32

	m, err := NewTokenManager(TokenManagerConfig{
		Cache:    cache,
		CacheKey: "token-key",
		Fetcher: func(ctx context.Context) (TokenFetchResult, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(30 * time.Millisecond)
			return TokenFetchResult{Token: "fresh", ExpiresIn: 7200}, nil
		},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			token, err := m.GetToken(context.Background())
			if err != nil {
				t.Errorf("get token: %v", err)
				return
			}
			if token != "fresh" {
				t.Errorf("unexpected token: %s", token)
			}
		})
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected one fetch call, got %d", got)
	}
}

func TestTokenManagerRefreshBypassesCache(t *testing.T) {
	cache := newTokenTestCache()
	_ = cache.Set(context.Background(), "token-key", "cached", 0)
	var calls int32

	m, err := NewTokenManager(TokenManagerConfig{
		Cache:    cache,
		CacheKey: "token-key",
		Fetcher: func(ctx context.Context) (TokenFetchResult, error) {
			atomic.AddInt32(&calls, 1)
			return TokenFetchResult{Token: "fresh", ExpiresIn: 7200}, nil
		},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	token, err := m.GetToken(context.Background())
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	if token != "cached" {
		t.Fatalf("unexpected cached token: %s", token)
	}

	token, err = m.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	if token != "fresh" {
		t.Fatalf("unexpected refreshed token: %s", token)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected one fetch call, got %d", got)
	}
}
