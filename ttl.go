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
	"encoding/json"
	"errors"
	"time"
)

type (
	Store interface {
		Set(ctx context.Context, key string, value []byte, ttl ...time.Duration) error
		Get(ctx context.Context, key string) ([]byte, error)
		Delete(ctx context.Context, key string) error
		Close(ctx context.Context) error
	}
	Cache struct {
		keyPrefix string
		ttl       time.Duration
		stores    []Store
	}

	Option struct {
		store            Store
		secondaryStore   Store
		keyPrefix        string
		cleanWindow      time.Duration
		maxEntrySize     int
		hardMaxCacheSize int
		onRemove         func(key string)
	}
)

var ErrNotFound = errors.New("Not found")

type Marshaler interface {
	Marshal() ([]byte, error)
}
type Unmarshaler interface {
	Unmarshal([]byte) error
}

// CacheOption cache option
type CacheOption func(opt *Option)

func CacheCleanWindowOption(cleanWindow time.Duration) CacheOption {
	return func(opt *Option) {
		opt.cleanWindow = cleanWindow
	}
}

func CacheMaxEntrySizeOption(maxEntrySize int) CacheOption {
	return func(opt *Option) {
		opt.maxEntrySize = maxEntrySize
	}
}

func CacheHardMaxCacheSizeOption(hardMaxCacheSize int) CacheOption {
	return func(opt *Option) {
		opt.hardMaxCacheSize = hardMaxCacheSize
	}
}

func CacheOnRemoveOption(onRemove func(key string)) CacheOption {
	return func(opt *Option) {
		opt.onRemove = onRemove
	}
}

func CacheKeyPrefixOption(keyPrefix string) CacheOption {
	return func(opt *Option) {
		opt.keyPrefix = keyPrefix
	}
}

func CacheStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.store = store
	}
}

func CacheSecondaryStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.secondaryStore = store
	}
}

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
		keyPrefix: opt.keyPrefix,
		ttl:       ttl,
		stores:    stores,
	}, nil
}

func (c *Cache) Close(ctx context.Context) error {
	for _, s := range c.stores {
		err := s.Close(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) getKey(key string) string {
	return c.keyPrefix + key
}

func (c *Cache) getTTL(ttl ...time.Duration) time.Duration {
	if len(ttl) != 0 {
		return ttl[0]
	}
	return c.ttl
}

func (c *Cache) getBytes(ctx context.Context, key string) ([]byte, error) {
	max := len(c.stores)
	for index, s := range c.stores {
		buf, err := s.Get(ctx, key)
		// 出错，而且是最后一个store
		// 则直接返回
		if err != nil && index == max-1 {
			return nil, err
		}
		// 如果获取到数据
		if len(buf) != 0 {
			return buf, nil
		}
	}
	return nil, nil
}

func (c *Cache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return c.getBytes(ctx, c.getKey(key))
}

func (c *Cache) setBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	for _, s := range c.stores {
		err := s.Set(ctx, key, value, ttl)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) SetBytes(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	return c.setBytes(ctx, c.getKey(key), value, c.getTTL(ttl...))
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl ...time.Duration) error {
	var entry []byte
	marshaler, ok := value.(Marshaler)
	// 如果本身支持marshal
	if ok {
		buf, err := marshaler.Marshal()
		if err != nil {
			return err
		}
		entry = buf
	} else {
		buf, err := json.Marshal(value)
		if err != nil {
			return err
		}
		entry = buf
	}
	return c.setBytes(ctx, c.getKey(key), entry, c.getTTL(ttl...))
}

func Get[T any](ctx context.Context, c *Cache, key string) (*T, error) {
	v := new(T)
	err := c.Get(ctx, key, v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Cache) Get(ctx context.Context, key string, value any) error {
	data, err := c.getBytes(ctx, c.getKey(key))
	if err != nil {
		return err
	}
	unmarshaler, ok := value.(Unmarshaler)
	if ok {
		return unmarshaler.Unmarshal(data)
	}
	return json.Unmarshal(data, value)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	key = c.getKey(key)
	var err error
	for _, s := range c.stores {
		e := s.Delete(ctx, key)
		// 无论是否出错均继续删除
		if e != nil {
			err = e
		}
	}
	return err
}
