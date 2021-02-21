# go-cache

[![Build Status](https://github.com/vicanso/go-cache/workflows/Test/badge.svg)](https://github.com/vicanso/go-cache/actions)

常用的缓存组件

## RedisCache

封装了一些常用的redis函数，保证所有缓存均需要指定ttl

## MultilevelCache

使用lru+redis来组合多层缓存
