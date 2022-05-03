// Copyright 2022 tree xie
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

	"github.com/allegro/bigcache/v3"
)

type bigCacheStore struct {
	client *bigcache.BigCache
}

func (bcs *bigCacheStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	return bcs.client.Set(key, value)
}

func (bcs *bigCacheStore) Get(_ context.Context, key string) ([]byte, error) {
	buf, err := bcs.client.Get(key)
	if err == bigcache.ErrEntryNotFound {
		err = ErrIsNil
	}
	return buf, err
}

func (bcs *bigCacheStore) Close(_ context.Context) error {
	return bcs.client.Close()
}

func (bcs *bigCacheStore) Delete(_ context.Context, key string) error {
	return bcs.client.Delete(key)
}

func newBigCacheStore(ttl time.Duration, opt *Option) (Store, error) {
	conf := bigcache.DefaultConfig(ttl)
	// 设置默认的shards
	// 因为常用的场景为初始化多个store
	// 因此默认的shard使用较小的值即可
	conf.Shards = 1 << 3
	if opt.cleanWindow > time.Second {
		conf.CleanWindow = opt.cleanWindow
	}
	if opt.maxEntrySize > 0 {
		conf.MaxEntrySize = opt.maxEntrySize
	}
	if opt.hardMaxCacheSize > 0 {
		conf.HardMaxCacheSize = opt.hardMaxCacheSize
	}
	if opt.onRemove != nil {
		conf.OnRemove = func(key string, _ []byte) {
			opt.onRemove(key)
		}
	}
	if opt.shards > 0 {
		conf.Shards = opt.shards
	}
	c, err := bigcache.NewBigCache(conf)
	if err != nil {
		return nil, err
	}
	return &bigCacheStore{
		client: c,
	}, nil
}
