# go-cache

[![Build Status](https://github.com/vicanso/go-cache/workflows/Test/badge.svg)](https://github.com/vicanso/go-cache/actions)

常用的缓存库，提供各类常用的缓存函数，通过简单的组合则可实现支持TTL、数据压缩的多级缓存。

## 错误语义

- 缓存未命中统一返回 `cache.ErrNotFound`（与历史名 `cache.ErrIsNil` 为同一 sentinel，可用 `errors.Is` 判断）。
- `RedisCache` 不再直接透传 `redis.Nil`，同样映射为 `ErrNotFound`。
- `RedisSession.Get` 仍按 session 语义：不存在时返回空数据与 `nil` error。

## 多级写入语义

`Cache.Set` / `SetBytes` 向所有 store **顺序、尽力写入**（best-effort），**不保证跨 store 原子性**。靠后的 store 失败时，靠前的 store 可能已更新；调用方如需强一致应自行补偿或只使用单 store。

读路径上，后级命中会回填前级；同一 key 的后级加载与回填通过 `singleflight` 合并，降低惊群。逻辑过期的一级条目会在读取时删除。

## Store

默认提供了两种常用的支持 ttl 的 store：

- `bigcache`: 基于 bigcache 的内存 store。实例级硬生命周期在使用默认 store 时取 **一级 TTL**（`CacheMultiTTLOption` 的 `ttlList[0]`，否则为 `New` 的 `ttl`），与写入时嵌入的软过期时间戳对齐。
- `redis`: 基于 redis 的 store，支持按 key 设置不同 ttl。

### Redis client 所有权

- `NewRedisStore(client)`：**不**在 `Close` 时关闭 client（适合共享连接）。
- `NewRedisStoreOwned(client)`：`Close` 时关闭 client（仅当 cache 独占该连接时使用）。

## 示例

```go
c, err := cache.New(
    10*time.Minute,
    // 数据清除时间窗口(仅用于bigcache)，默认为1秒
    cache.CacheCleanWindowOption(5*time.Second),
    // 最大的entry大小，用于缓存的初始大小，默认为500(仅用于bigcache)
    cache.CacheMaxEntrySizeOption(1024),
    // 最大使用的内存大小(MB)，默认为0不限制(仅用于bigcache)
    cache.CacheHardMaxCacheSizeOption(10),
    // 指定key的前缀
    cache.CacheKeyPrefixOption("prefix:"),
    // 指定二级缓存（共享 redis client 时请用 NewRedisStore，勿用 Owned）
    cache.CacheSecondaryStoreOption(cache.NewRedisStore(redisClient)),
    // 指定不同的缓存使用不同的ttl；默认 bigcache 硬生命周期使用一级 TTL
    cache.CacheMultiTTLOption([]time.Duration{
        // 一级缓存有效期
        time.Minute,
        // 二级缓存有效期
        10 * time.Minute,
    }),
)
if err != nil {
    // ...
}
defer c.Close(context.Background())
```

## RedisCache

封装了一些常用的 redis 函数，保证所有缓存均需要指定 ttl，并支持指定前缀。

### 指定缓存 Key 的前缀

```go
client := newRedisClient()
opts := []cache.RedisCacheOption{
    cache.RedisCachePrefixOption("prefix:"),
}
c := cache.NewRedisCache(client, opts...)
```

### 分布式锁（token）

`Lock` 使用唯一 token 作为锁值；释放时必须携带同一 token（`Unlock` 或 `LockWithDone` 返回的 `done`），避免 TTL 过期后误删后来者的锁。

```go
ok, token, err := c.Lock(ctx, "resource", 5*time.Second)
if err != nil || !ok {
    return err
}
defer c.Unlock(ctx, "resource", token)

// 或
ok, done, err := c.LockWithDone(ctx, "resource", 5*time.Second)
if ok {
    defer done()
}
```

### Redis Session

用于 elton 中 session 的 redis 缓存。

```go
client := newRedisClient()
ss := cache.NewRedisSession(client)
ss.SetPrefix(redisConfig.Prefix + "ss:")
```
