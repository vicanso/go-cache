// Copyright 2021 tree xie
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	lruttl "github.com/vicanso/lru-ttl"
)

const multilevelCacheDefaultTimeout = 3 * time.Second
const multilevelCacheDefaultLRUSize = 100

type slowCache struct {
	cache   *RedisCache
	timeout time.Duration
}

type MultilevelCacheOption func(opt *multilevelCacheOptions)
type multilevelCacheOptions struct {
	Cache   *RedisCache
	LRUSize int
	TTL     time.Duration
	Timeout time.Duration
	Prefix  string
}

// MultilevelCacheRedisOption sets redis option
func MultilevelCacheRedisOption(c *RedisCache) MultilevelCacheOption {
	return func(opt *multilevelCacheOptions) {
		opt.Cache = c
	}
}

// MultilevelCacheLRUSizeOption sets lru size option
func MultilevelCacheLRUSizeOption(size int) MultilevelCacheOption {
	return func(opt *multilevelCacheOptions) {
		opt.LRUSize = size
	}
}

// MultilevelCacheTTLOption sets ttl option
func MultilevelCacheTTLOption(ttl time.Duration) MultilevelCacheOption {
	return func(opt *multilevelCacheOptions) {
		opt.TTL = ttl
	}
}

// MultilevelCacheTimeoutOption sets timeout option
func MultilevelCacheTimeoutOption(timeout time.Duration) MultilevelCacheOption {
	return func(opt *multilevelCacheOptions) {
		opt.Timeout = timeout
	}
}

// MultilevelCachePrefixOption sets prefix option
func MultilevelCachePrefixOption(prefix string) MultilevelCacheOption {
	return func(opt *multilevelCacheOptions) {
		opt.Prefix = prefix
	}
}

// Get cache from redis, it will return lruttl.ErrIsNil if data is not exists
func (sc *slowCache) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sc.timeout)
	defer cancel()
	buf, err := sc.cache.Get(ctx, key)
	// 转换redis nil error 为lruttl 的err is nil
	if err == redis.Nil {
		err = lruttl.ErrIsNil
	}
	return buf, err
}

// Set cache to redis with ttl
func (sc *slowCache) Set(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), sc.timeout)
	defer cancel()
	return sc.cache.Set(ctx, key, value, ttl)
}

// NewMultilevelCache returns a new multilevel cache,
// it will panic if Cache is nil or TTL is < one second
func NewMultilevelCache(opts ...MultilevelCacheOption) *lruttl.L2Cache {
	multiOptions := multilevelCacheOptions{}
	for _, opt := range opts {
		opt(&multiOptions)
	}

	if multiOptions.Cache == nil {
		panic("cache can not be nil")
	}
	if multiOptions.TTL < time.Second {
		panic("ttl can not lt 1s")
	}
	size := multilevelCacheDefaultLRUSize
	if multiOptions.LRUSize > 0 {
		size = multiOptions.LRUSize
	}

	timeout := multilevelCacheDefaultTimeout
	if multiOptions.Timeout > 0 {
		timeout = multiOptions.Timeout
	}
	cacheOpts := make([]lruttl.L2CacheOption, 0)
	if multiOptions.Prefix != "" {
		cacheOpts = append(cacheOpts, lruttl.L2CachePrefixOption(multiOptions.Prefix))
	}
	l2 := lruttl.NewL2Cache(&slowCache{
		timeout: timeout,
		cache:   multiOptions.Cache,
	}, size, multiOptions.TTL, cacheOpts...)
	return l2
}
