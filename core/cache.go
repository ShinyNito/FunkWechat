package core

import (
	"context"
	"time"
)

// Cache 存放 access_token、ticket 等短期凭证。实现需要并发安全。
type Cache interface {
	// Get 返回未过期的值；不存在或已过期时返回 false。
	Get(ctx context.Context, key string) (string, bool)
	// Set 写入值，ttl 为 0 表示永不过期。
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	// Delete 删除值，key 不存在时返回 nil。
	Delete(ctx context.Context, key string) error
}
