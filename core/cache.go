package core

import (
	"context"
	"time"
)

// Cache 通用缓存接口
// 提供基础的字符串读写与删除能力，调用方可以用来缓存 access token 等短期数据。
type Cache interface {
	// Get 获取缓存值
	// 根据 key 读取缓存中的字符串值，若不存在或已过期则返回空字符串与 false。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 缓存键
	//
	// 返回:
	//   - string: 命中时的缓存值，未命中时为空字符串
	//   - bool: 是否命中缓存
	//
	// 错误:
	//   - 无: 该方法不返回 error
	Get(ctx context.Context, key string) (string, bool)

	// Set 写入缓存值
	// 将字符串值写入缓存并设置 TTL，TTL 为 0 时表示永不过期。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 缓存键
	//   - value: 要写入的字符串值
	//   - ttl: 生存时间，0 表示永不过期
	//
	// 返回:
	//   - error: 可能的错误
	//
	// 错误:
	//   - 底层存储写入失败
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// Delete 删除缓存值
	// 删除指定 key 的缓存项，若 key 不存在应静默成功。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 缓存键
	//
	// 返回:
	//   - error: 可能的错误
	//
	// 错误:
	//   - 底层存储删除失败
	Delete(ctx context.Context, key string) error
}
