# go-cache

[![Build Status](https://github.com/vicanso/go-cache/workflows/Test/badge.svg)](https://github.com/vicanso/go-cache/actions)

常用的缓存组件

## RedisCache

封装了一些常用的redis函数，保证所有缓存均需要指定ttl，并支持前缀形式、压缩以及二级缓存的处理。


### 指定缓存Key的前缀
```go
// 创建一个redis的client
client := newRedisClient()
opts := []goCache.RedisCacheOption{
    goCache.RedisCachePrefixOption("prefix:"),
}
c := goCache.NewRedisCache(client, opts...)
```

### 指定数据压缩

```go
// 创建一个redis的client
client := newRedisClient()
opts := []goCache.RedisCacheOption{
    goCache.RedisCachePrefixOption("prefix:"),
}
c := goCache.NewRedisCache(client, opts...)
// 大于10KB以上的数据压缩
// 适用于数据量较大，而且数据内容重复较多的场景
minCompressSize := 10 * 1024
return goCache.NewCompressRedisCache(
    client,
    minCompressSize,
    goCache.RedisCachePrefixOption("prefix:"),
)
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

## MultilevelCache

使用lru+redis来组合多层缓存。


```go
// 创建一个redis的client
client := newRedisClient()
c := goCache.NewRedisCache(client)
opts := []goCache.MultilevelCacheOption{
    goCache.MultilevelCacheRedisOption(c),
    goCache.MultilevelCacheLRUSizeOption(1024),
    goCache.MultilevelCacheTTLOption(5 * time.Minute),
    goCache.MultilevelCachePrefixOption("prefix:"),
}
return goCache.NewMultilevelCache(opts...)
```
