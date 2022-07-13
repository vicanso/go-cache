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
	ttlList    []time.Duration
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
	ttlList := opt.ttlList
	if len(ttlList) == 0 {
		ttlList = []time.Duration{
			ttl,
		}
	}

	return &Cache{
		compressor: opt.compressor,
		keyPrefix:  opt.keyPrefix,
		ttlList:    ttlList,
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

func (c *Cache) getTTL(index int, ttl ...time.Duration) time.Duration {
	if len(ttl) != 0 {
		return ttl[0]
	}
	if len(c.ttlList) > index {
		return c.ttlList[index]
	}
	return c.ttlList[0]
}

func (c *Cache) getBytes(ctx context.Context, key string) ([]byte, time.Duration, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, 0, err
	}

	max := len(c.stores)
	var data []byte
	var expiredAt time.Time
	now := time.Now()
	var ttl time.Duration
	for index, s := range c.stores {
		buf, err := s.Get(ctx, key)
		// 出错，而且是最后一个store
		// 则直接返回
		if err != nil && index == max-1 {
			return nil, 0, err
		}
		// 如果获取到数据
		if len(buf) >= timestampByteSize {
			expiredAt = getTimeFromBytes(buf)
			// 如果已过期，继续查询
			ttl = expiredAt.Sub(now)
			if ttl < 0 {
				continue
			}
			// 第一个store的数据已过期，将数据重新设置至store
			// 一般情况下index为0，由于bigcache可能因为空间不足导致数据清除
			// 或者二级缓存是redis，其它实例有操作更新
			if index != 0 {
				// 如果当前缓存对应的ttl
				// 少于第一个缓存的ttl(内存缓存有效期有可能较短），则
				// 使用新的ttl来修改记录
				firstIndex := 0
				newTTL := c.getTTL(firstIndex, ttl)
				if newTTL < ttl {
					ttl = newTTL
					writeTimeToBytes(time.Now().Add(ttl), data)
				}
				// 设置失败则忽略
				_ = c.stores[firstIndex].Set(ctx, key, buf, ttl)
			}
			data = buf[timestampByteSize:]
			break
		}
	}
	if len(data) == 0 {
		return nil, 0, ErrIsNil
	}

	// 如果有配置压缩
	if c.compressor != nil {
		buf, err := c.compressor.Decode(data)
		if err != nil {
			return nil, 0, err
		}
		data = buf
	}
	return data, ttl, nil
}

// GetBytes gets the data from cache
func (c *Cache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	buf, _, err := c.getBytes(ctx, key)
	return buf, err
}

// GetBytesAndTTL gets the data from cache and the ttl of data
func (c *Cache) GetBytesAndTTL(ctx context.Context, key string) ([]byte, time.Duration, error) {
	return c.getBytes(ctx, key)
}

func (c *Cache) setBytes(ctx context.Context, key string, value []byte, ttls ...time.Duration) error {
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
	// 增加ttl至value中
	data := make([]byte, len(value)+timestampByteSize)
	copy(data[timestampByteSize:], value)
	for index, s := range c.stores {
		ttl := c.getTTL(index, ttls...)
		writeTimeToBytes(time.Now().Add(ttl), data)
		err := s.Set(ctx, key, data, ttl)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetBytes sets the data to cache
func (c *Cache) SetBytes(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	return c.setBytes(ctx, key, value, ttl...)
}

// Set marshals the value to bytes and sets to cache
func (c *Cache) Set(ctx context.Context, key string, value any, ttl ...time.Duration) error {
	entry, err := marshal(value)
	if err != nil {
		return err
	}
	return c.setBytes(ctx, key, entry, ttl...)
}

func Get[T any](ctx context.Context, c *Cache, key string) (*T, error) {
	v := new(T)
	err := c.Get(ctx, key, v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Get gets the value from cache and unmarshals it
func (c *Cache) Get(ctx context.Context, key string, value any) error {
	data, _, err := c.getBytes(ctx, key)
	if err != nil {
		return err
	}
	return unmarshal(data, value)
}

// GetAndTTL gets the value from cache and unmarshals it, and returns the ttl of value
func (c *Cache) GetAndTTL(ctx context.Context, key string, value any) (time.Duration, error) {
	data, ttl, err := c.getBytes(ctx, key)
	if err != nil {
		return 0, err
	}
	err = unmarshal(data, value)
	if err != nil {
		return 0, err
	}
	return ttl, nil
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
