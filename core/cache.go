package core

import (
	"context"
	"time"
)

// Cache 可选的外部凭证存储，用于多个进程共享 access_token 和 ticket，例如基于 Redis 实现。
// 实现需要并发安全。
//
// 不配置时凭证只保存在进程内存里。配置后 SDK 仍然优先读内存，不会每个请求都读一次 Cache：
// 进程启动和凭证到期时读取；从 Cache 读到的值不知道剩余有效期，在内存里保留 1 分钟后重新读取，
// 所以凭证有效期内每分钟最多读一次。向微信获取到新凭证后写入 Cache。
type Cache interface {
	// Get 返回未过期的值。不存在或已过期时 ok 为 false；
	// 存储本身出错时返回 err，SDK 会记录日志并改为向微信获取。
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	// Set 写入值，ttl 始终大于 0。
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}
