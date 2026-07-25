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
		s, err := NewBigCacheStore(storeTTL)
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
// (e.g. zstd), it is closed as well. Every store and the compressor are always
// closed even when one of them fails, so a single failure cannot leak the
// others; all errors are joined into the returned error.
func (c *Cache) Close(ctx context.Context) error {
	var errs []error
	for _, s := range c.stores {
		if err := s.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if closer, ok := c.compressor.(Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

// decodePayload strips the compress flag written by encodeValue and
// decompresses the body when needed. Entries written without a compressor have
// no flag byte, which is why the flag is only read when one is configured.
func (c *Cache) decodePayload(data []byte, ttl time.Duration) ([]byte, time.Duration, error) {
	if c.compressor == nil || len(data) == 0 {
		return data, ttl, nil
	}
	if data[0] == CompressNone {
		return data[1:], ttl, nil
	}
	buf, err := c.compressor.DecodeTo(nil, data[1:])
	if err != nil {
		return nil, 0, err
	}
	return buf, ttl, nil
}

// encodeValue builds a store entry in a single allocation: an 8 byte expiry
// placeholder (filled in per store by setBytes) followed by the value. When a
// compressor is configured a 1 byte compress flag sits between the two.
func (c *Cache) encodeValue(value []byte) ([]byte, error) {
	if c.compressor == nil {
		data := make([]byte, timestampByteSize+len(value))
		copy(data[timestampByteSize:], value)
		return data, nil
	}
	const headerSize = timestampByteSize + 1
	if !c.compressor.Match(len(value)) {
		data := make([]byte, headerSize+len(value))
		data[timestampByteSize] = CompressNone
		copy(data[headerSize:], value)
		return data, nil
	}
	// MaxEncodedLen仅用于预分配，实现返回异常值时回退为原始长度
	size := c.compressor.MaxEncodedLen(len(value))
	if size < 0 {
		size = len(value)
	}
	data := make([]byte, headerSize, headerSize+size)
	data[timestampByteSize] = Compressed
	return c.compressor.EncodeTo(data, value)
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
	data, err := c.encodeValue(value)
	if err != nil {
		return err
	}
	// Best-effort multi-write: each store gets its own clone and TTL/timestamp.
	// 每个store都会被尝试写入(失败不中断也不回滚)，所有错误合并返回，
	// 避免出现"前面的store已写、后面的store未写"这种取决于遍历顺序的状态。
	// data由本函数新分配且不再被复用，最后一个store可直接接管，
	// 因此单store(常见场景)时无需任何克隆。
	now := time.Now()
	last := len(c.stores) - 1
	var errs []error
	for index, s := range c.stores {
		ttl := c.getTTL(index, ttls...)
		entry := data
		if index != last {
			entry = append([]byte(nil), data...)
		}
		writeTimeToBytes(now.Add(ttl), entry)
		if err := s.Set(ctx, key, entry, ttl); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SetBytes sets the data to cache. When multiple stores are configured, the
// write is best-effort across stores (not atomic): every store is attempted
// even if an earlier one fails, so a failure may leave the stores inconsistent.
// All store errors are joined into the returned error.
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
// signal a missing key). Every store is attempted even when one fails, and all
// other errors are joined into the returned error.
func (c *Cache) Delete(ctx context.Context, key string) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range c.stores {
		// 无论是否出错均继续删除其它store
		e := s.Delete(ctx, key)
		if e != nil && !errors.Is(e, ErrNotFound) {
			errs = append(errs, e)
		}
	}
	return errors.Join(errs...)
}
