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
	"errors"
	"time"
)

const (
	// CompressNone compress none
	CompressNone byte = iota
	// Compressed compress
	Compressed
)

type Cache struct {
	keyPrefix  string
	ttl        time.Duration
	stores     []Store
	compressor Compressor
}

var ErrIsNil = errors.New("Data is nil")
var ErrKeyIsNil = errors.New("Key is nil")

// New creates a new cache with default ttl
func New(ttl time.Duration, opts ...CacheOption) (*Cache, error) {
	opt := Option{}
	for _, fn := range opts {
		fn(&opt)
	}
	store := opt.store
	// 如果未指定store，则使用big cache
	if store == nil {
		s, err := newBigCacheStore(ttl, &opt)
		if err != nil {
			return nil, err
		}
		store = s
	}

	stores := []Store{
		store,
	}
	if opt.secondaryStore != nil {
		stores = append(stores, opt.secondaryStore)
	}

	return &Cache{
		compressor: opt.compressor,
		keyPrefix:  opt.keyPrefix,
		ttl:        ttl,
		stores:     stores,
	}, nil
}

// Close closes all stores of cache
func (c *Cache) Close(ctx context.Context) error {
	for _, s := range c.stores {
		err := s.Close(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) getKey(key string) (string, error) {
	if key == "" {
		return "", ErrKeyIsNil
	}
	return c.keyPrefix + key, nil
}

func (c *Cache) getTTL(ttl ...time.Duration) time.Duration {
	if len(ttl) != 0 {
		return ttl[0]
	}
	return c.ttl
}

func (c *Cache) getBytes(ctx context.Context, key string) ([]byte, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, err
	}

	max := len(c.stores)
	var data []byte
	for index, s := range c.stores {
		buf, err := s.Get(ctx, key)
		// 出错，而且是最后一个store
		// 则直接返回
		if err != nil && index == max-1 {
			return nil, err
		}
		// 如果获取到数据
		if len(buf) != 0 {
			data = buf
			break
		}
	}
	// 如果有配置压缩
	if c.compressor != nil {
		buf, err := c.compressor.Decode(data)
		if err != nil {
			return nil, err
		}
		data = buf
	}
	return data, nil
}

// GetBytes gets the data from cache
func (c *Cache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return c.getBytes(ctx, key)
}

func (c *Cache) setBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	// 如果有设置解压
	if c.compressor != nil {
		buf, err := c.compressor.Encode(value)
		if err != nil {
			return err
		}
		value = buf
	}
	for _, s := range c.stores {
		err := s.Set(ctx, key, value, ttl)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetBytes sets the data to cache
func (c *Cache) SetBytes(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	return c.setBytes(ctx, key, value, c.getTTL(ttl...))
}

// Set marshals the value to bytes and sets to cache
func (c *Cache) Set(ctx context.Context, key string, value any, ttl ...time.Duration) error {
	entry, err := marshal(value)
	if err != nil {
		return err
	}
	return c.setBytes(ctx, key, entry, c.getTTL(ttl...))
}

func Get[T any](ctx context.Context, c *Cache, key string) (*T, error) {
	v := new(T)
	err := c.Get(ctx, key, v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Get gets the value for cache and unmarshals it
func (c *Cache) Get(ctx context.Context, key string, value any) error {
	data, err := c.getBytes(ctx, key)
	if err != nil {
		return err
	}
	return unmarshal(data, value)
}

// Delete deletes all the data from all stores
func (c *Cache) Delete(ctx context.Context, key string) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	for _, s := range c.stores {
		e := s.Delete(ctx, key)
		// 无论是否出错均继续删除
		if e != nil {
			err = e
		}
	}
	return err
}
