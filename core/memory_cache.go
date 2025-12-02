package core

import (
	"context"
	"sync"
	"time"
)

// cacheItem 缓存项
type cacheItem struct {
	value     string
	expiresAt time.Time
}

// isExpired 判断缓存项是否过期
func (item *cacheItem) isExpired() bool {
	if item.expiresAt.IsZero() {
		return false // 永不过期
	}
	return time.Now().After(item.expiresAt)
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]*cacheItem),
	}
}

// Get 获取缓存值
func (c *MemoryCache) Get(ctx context.Context, key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return "", false
	}

	if item.isExpired() {
		return "", false
	}

	return item.value, true
}

// Set 设置缓存值
func (c *MemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	return nil
}

// Delete 删除缓存
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Cleanup 清理过期缓存项（可选，用于定期清理）
func (c *MemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.isExpired() {
			delete(c.items, key)
		}
	}
}

// 确保 MemoryCache 实现了 Cache 接口
var _ Cache = (*MemoryCache)(nil)
