# go-cache

[![Build Status](https://github.com/vicanso/go-cache/workflows/Test/badge.svg)](https://github.com/vicanso/go-cache/actions)

常用的缓存库，提供各类常用的缓存函数，通过简单的组合则可实现支持TTL、数据压缩的多级缓存

## Store

默认提供了两种常用的支持ttl的store：

- `bigcache`: 基于bigcache的内存store，但仅支持实例初始化时指定ttl，不可每个key设置不同的ttl
- `redis`: 基于redis的store，支持实例化时指定默认的ttl，并可针对不同的key设置不同的ttl

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
    // 指定二级缓存
    cache.CacheSecondaryStoreOption(cache.NewRedisStore(redisClient)),
)
```

## RedisCache

封装了一些常用的redis函数，保证所有缓存均需要指定ttl，并支持指定前缀。


### 指定缓存Key的前缀
```go
// 创建一个redis的client
client := newRedisClient()
opts := []goCache.RedisCacheOption{
    goCache.RedisCachePrefixOption("prefix:"),
}
c := goCache.NewRedisCache(client, opts...)
```


### Redis Session

用于elton中session的redis缓存。

```go
// 创建一个redis的client
client := newRedisClient()
ss := goCache.NewRedisSession(client)
// 设置前缀
ss.SetPrefix(redisConfig.Prefix + "ss:")
```
