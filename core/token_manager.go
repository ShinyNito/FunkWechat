package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const defaultExpireBufferSeconds = 300

type TokenFetchResult struct {
	Token     string
	ExpiresIn int
}

type TokenFetcher func(ctx context.Context) (TokenFetchResult, error)

type TokenManagerConfig struct {
	Cache               Cache
	CacheKey            string
	Fetcher             TokenFetcher
	Logger              *slog.Logger
	ExpireBufferSeconds int
}

type tokenCall struct {
	done  chan struct{}
	token string
	err   error
}

type TokenManager struct {
	cache               Cache
	cacheKey            string
	fetcher             TokenFetcher
	logger              *slog.Logger
	expireBufferSeconds int

	mu       sync.Mutex
	inflight *tokenCall
}

func NewTokenManager(cfg TokenManagerConfig) (*TokenManager, error) {
	if cfg.Cache == nil {
		return nil, fmt.Errorf("cache is required")
	}
	if cfg.CacheKey == "" {
		return nil, fmt.Errorf("cache key is required")
	}
	if cfg.Fetcher == nil {
		return nil, fmt.Errorf("fetcher is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	expireBufferSeconds := cfg.ExpireBufferSeconds
	if expireBufferSeconds <= 0 {
		expireBufferSeconds = defaultExpireBufferSeconds
	}

	return &TokenManager{
		cache:               cfg.Cache,
		cacheKey:            cfg.CacheKey,
		fetcher:             cfg.Fetcher,
		logger:              logger,
		expireBufferSeconds: expireBufferSeconds,
	}, nil
}

func (m *TokenManager) GetToken(ctx context.Context) (string, error) {
	if token, ok := m.cache.Get(ctx, m.cacheKey); ok {
		return token, nil
	}
	return m.do(ctx, false)
}

func (m *TokenManager) RefreshToken(ctx context.Context) (string, error) {
	return m.do(ctx, true)
}

func (m *TokenManager) do(ctx context.Context, force bool) (string, error) {
	m.mu.Lock()
	if !force {
		if token, ok := m.cache.Get(ctx, m.cacheKey); ok {
			m.mu.Unlock()
			return token, nil
		}
	}

	if m.inflight != nil {
		call := m.inflight
		m.mu.Unlock()
		return waitTokenCall(ctx, call)
	}

	call := &tokenCall{done: make(chan struct{})}
	m.inflight = call
	m.mu.Unlock()

	token, err := m.fetchAndStore(ctx, force)
	call.token = token
	call.err = err
	close(call.done)

	m.mu.Lock()
	if m.inflight == call {
		m.inflight = nil
	}
	m.mu.Unlock()

	return token, err
}

func (m *TokenManager) fetchAndStore(ctx context.Context, force bool) (string, error) {
	if !force {
		if token, ok := m.cache.Get(ctx, m.cacheKey); ok {
			return token, nil
		}
	}

	result, err := m.fetcher(ctx)
	if err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", fmt.Errorf("empty token from fetcher")
	}

	ttlSeconds := max(result.ExpiresIn-m.expireBufferSeconds, 1)
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := m.cache.Set(ctx, m.cacheKey, result.Token, ttl); err != nil {
		m.logger.WarnContext(ctx, "cache token failed", slog.String("key", m.cacheKey), slog.Any("error", err))
	}

	return result.Token, nil
}

func waitTokenCall(ctx context.Context, call *tokenCall) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-call.done:
		return call.token, call.err
	}
}

var _ AccessTokenProvider = (*TokenManager)(nil)
