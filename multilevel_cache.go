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

type MultilevelCacheOptions struct {
	Cache   *RedisCache
	LRUSize int
	TTL     time.Duration
	Timeout time.Duration
	Prefix  string
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
func NewMultilevelCache(opts MultilevelCacheOptions) *lruttl.L2Cache {
	if opts.Cache == nil {
		panic("cache can not be nil")
	}
	if opts.TTL < time.Second {
		panic("ttl can not lt 1s")
	}
	size := opts.LRUSize
	if size <= 0 {
		size = multilevelCacheDefaultLRUSize
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = multilevelCacheDefaultTimeout
	}
	cacheOpts := make([]lruttl.L2CacheOption, 0)
	if opts.Prefix != "" {
		cacheOpts = append(cacheOpts, lruttl.L2CachePrefixOption(opts.Prefix))
	}
	l2 := lruttl.NewL2Cache(&slowCache{
		timeout: timeout,
		cache:   opts.Cache,
	}, size, opts.TTL, cacheOpts...)
	return l2
}
