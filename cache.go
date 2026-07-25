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

	"golang.org/x/sync/singleflight"
)

const (
	// CompressNone compress none
	CompressNone byte = iota
	// Compressed compress
	Compressed
)

// Cache is a multi-level cache. Writes fan out to every configured store
// (best-effort, not atomic across stores). Reads walk stores in order and
// back-fill earlier tiers on a later hit.
type Cache struct {
	keyPrefix  string
	ttlList    []time.Duration
	stores     []Store
	compressor Compressor
	// sfg coalesces concurrent secondary loads + primary back-fills for the same key
	sfg singleflight.Group
}

// ErrIsNil is returned when the key is missing or the entry is logically expired.
// Prefer ErrNotFound at new call sites; both values are identical for errors.Is.
var ErrIsNil = errors.New("data is nil")

// ErrNotFound is an alias of ErrIsNil with clearer semantics for cache misses.
var ErrNotFound = ErrIsNil

// ErrKeyIsNil is returned when the cache key is empty.
var ErrKeyIsNil = errors.New("key is nil")

// New creates a new cache with default ttl.
// When CacheMultiTTLOption is set and the default bigcache store is used, the
// primary TTL (ttlList[0]) is used for bigcache's hard lifetime so it stays
// aligned with the soft per-key TTL embedded in values.
func New(ttl time.Duration, opts ...CacheOption) (*Cache, error) {
	opt := Option{}
	for _, fn := range opts {
		fn(&opt)
	}
	store := opt.store
	// Prefer multi-TTL primary duration for the default in-memory store so
	// hard eviction matches the soft TTL (avoids long-lived logical zombies).
	storeTTL := ttl
	if len(opt.ttlList) > 0 {
		storeTTL = opt.ttlList[0]
	}
	// 如果未指定store，则使用big cache
	if store == nil {
		s, err := newBigCacheStore(storeTTL, &opt)
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

// Close closes all stores of cache. If the compressor implements Closer
// (e.g. zstd), it is closed as well.
func (c *Cache) Close(ctx context.Context) error {
	for _, s := range c.stores {
		err := s.Close(ctx)
		if err != nil {
			return err
		}
	}
	if closer, ok := c.compressor.(Closer); ok {
		if err := closer.Close(); err != nil {
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

// cachePayload is the in-flight result of a store read (value without timestamp).
type cachePayload struct {
	data []byte
	ttl  time.Duration
}

// readStore loads one store entry. On logical expiry it optionally deletes the
// stale entry so subsequent reads skip the zombie. Non-miss errors are returned
// as-is; misses (ErrIsNil/ErrNotFound or short payloads) yield found=false.
func (c *Cache) readStore(ctx context.Context, index int, key string, deleteExpired bool) ([]byte, time.Duration, bool, error) {
	buf, err := c.stores[index].Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}
	if len(buf) < timestampByteSize {
		return nil, 0, false, nil
	}
	expiredAt := getTimeFromBytes(buf)
	ttl := time.Until(expiredAt)
	if ttl < 0 {
		if deleteExpired {
			_ = c.stores[index].Delete(ctx, key)
		}
		return nil, 0, false, nil
	}
	return buf, ttl, true, nil
}

// backfillPrimary writes a secondary hit into the primary store, capping TTL
// to the primary configuration when needed. The buffer is always cloned so
// stores never share backing arrays.
func (c *Cache) backfillPrimary(ctx context.Context, key string, buf []byte, ttl time.Duration) {
	entry := append([]byte(nil), buf...)
	primaryTTL := c.getTTL(0)
	if primaryTTL < ttl {
		ttl = primaryTTL
		writeTimeToBytes(time.Now().Add(ttl), entry)
	}
	_ = c.stores[0].Set(ctx, key, entry, ttl)
}

func (c *Cache) decodePayload(data []byte, ttl time.Duration) ([]byte, time.Duration, error) {
	if c.compressor != nil {
		buf, err := c.compressor.Decode(data)
		if err != nil {
			return nil, 0, err
		}
		data = buf
	}
	return data, ttl, nil
}

func (c *Cache) getBytes(ctx context.Context, key string) ([]byte, time.Duration, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, 0, err
	}

	// Fast path: primary store only
	buf, ttl, found, err := c.readStore(ctx, 0, key, true)
	if err != nil {
		// Primary hard error with no secondary: surface it
		if len(c.stores) == 1 {
			return nil, 0, err
		}
		// Otherwise fall through to secondary (resilient multi-level read)
	} else if found {
		return c.decodePayload(buf[timestampByteSize:], ttl)
	}

	if len(c.stores) <= 1 {
		return nil, 0, ErrNotFound
	}

	// Coalesce concurrent secondary loads + back-fills for this key
	v, err, _ := c.sfg.Do(key, func() (any, error) {
		// Another goroutine may have already back-filled primary
		buf, ttl, found, err := c.readStore(ctx, 0, key, true)
		if err == nil && found {
			return &cachePayload{data: buf[timestampByteSize:], ttl: ttl}, nil
		}

		max := len(c.stores)
		for index := 1; index < max; index++ {
			buf, ttl, found, err := c.readStore(ctx, index, key, true)
			if err != nil {
				if index == max-1 {
					if errors.Is(err, ErrNotFound) {
						return nil, ErrNotFound
					}
					return nil, err
				}
				continue
			}
			if !found {
				continue
			}
			c.backfillPrimary(ctx, key, buf, ttl)
			return &cachePayload{data: buf[timestampByteSize:], ttl: ttl}, nil
		}
		return nil, ErrNotFound
	})
	if err != nil {
		return nil, 0, err
	}
	p := v.(*cachePayload)
	return c.decodePayload(p.data, p.ttl)
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
	// 构造 [时间戳前缀][(可选)压缩数据]。若compressor支持预留前缀
	// (内置实现)，则时间戳前缀与压缩头合并为一次内存分配；
	// 否则回退为先压缩再拼接前缀。
	var data []byte
	if c.compressor == nil {
		data = make([]byte, len(value)+timestampByteSize)
		copy(data[timestampByteSize:], value)
	} else if pe, ok := c.compressor.(prefixEncoder); ok {
		data, err = pe.encodeWithPrefix(timestampByteSize, value)
		if err != nil {
			return err
		}
	} else {
		buf, err := c.compressor.Encode(value)
		if err != nil {
			return err
		}
		data = make([]byte, len(buf)+timestampByteSize)
		copy(data[timestampByteSize:], buf)
	}
	// Best-effort multi-write: each store gets its own clone and TTL/timestamp.
	// Failure on a later store does not roll back earlier stores.
	// data由本函数新分配且不再被复用，最后一个store可直接接管，
	// 因此单store(常见场景)时无需任何克隆。
	now := time.Now()
	last := len(c.stores) - 1
	for index, s := range c.stores {
		ttl := c.getTTL(index, ttls...)
		entry := data
		if index != last {
			entry = append([]byte(nil), data...)
		}
		writeTimeToBytes(now.Add(ttl), entry)
		err := s.Set(ctx, key, entry, ttl)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetBytes sets the data to cache. When multiple stores are configured, the
// write is best-effort across stores (not atomic); a failure may leave earlier
// stores updated.
func (c *Cache) SetBytes(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	return c.setBytes(ctx, key, value, ttl...)
}

// Set marshals the value to bytes and sets to cache. Multi-store writes are
// best-effort (see SetBytes).
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

// Delete deletes the data from all stores. Deleting an absent key is a no-op:
// ErrNotFound from a store is not reported (stores differ on whether they
// signal a missing key). Every store is attempted even when one fails, and the
// last non-nil error is returned.
func (c *Cache) Delete(ctx context.Context, key string) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	var lastErr error
	for _, s := range c.stores {
		// 无论是否出错均继续删除其它store
		e := s.Delete(ctx, key)
		if e != nil && !errors.Is(e, ErrNotFound) {
			lastErr = e
		}
	}
	return lastErr
}
