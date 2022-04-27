# go-cache

[![Build Status](https://github.com/vicanso/go-cache/workflows/Test/badge.svg)](https://github.com/vicanso/go-cache/actions)

常用的缓存库，提供各类常用的缓存函数，通过简单的组合则可实现支持TTL、数据压缩的多级缓存

## Store

默认提供了两种常用的支持ttl的store：

- `bigcache`: 基于bigcache的内存store，但仅支持实例初始化时指定ttl，不可每个key设置不同的ttl
- `redis`: 基于redis的store，支持实例化时指定默认的ttl，并可针对不同的key设置不同的ttl

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


### Redis Session

用于elton中session的redis缓存。

```go
// 创建一个redis的client
client := newRedisClient()
ss := goCache.NewRedisSession(client)
// 设置前缀
ss.SetPrefix(redisConfig.Prefix + "ss:")
```
