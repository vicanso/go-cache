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

	lruttl "github.com/vicanso/lru-ttl"
)

const multilevelCacheDefaultTimeout = 3 * time.Second
const multilevelCacheDefaultLRUSize = 100

type slowCache struct {
	cache *redisCache
}

type MultilevelCacheOptions struct {
	Cache   *redisCache
	LRUSize int
	TTL     time.Duration
	Prefix  string
}

func (sc *slowCache) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), multilevelCacheDefaultTimeout)
	defer cancel()
	return sc.cache.Get(ctx, key)
}

func (sc *slowCache) Set(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), multilevelCacheDefaultTimeout)
	defer cancel()
	return sc.cache.Set(ctx, key, value, ttl)
}

// NewMultilevelCache create a multilevel cache
func NewMultilevelCache(opts MultilevelCacheOptions) *lruttl.L2Cache {
	if opts.Cache == nil {
		panic("cache can not be nil")
	}
	if opts.TTL < time.Second {
		panic("ttl cat not lt 1s")
	}
	size := opts.LRUSize
	if size <= 0 {
		size = multilevelCacheDefaultLRUSize
	}

	l2 := lruttl.NewL2Cache(&slowCache{
		cache: opts.Cache,
	}, size, opts.TTL)
	if opts.Prefix != "" {
		l2.SetPrefix(opts.Prefix)
	}
	return l2
}
