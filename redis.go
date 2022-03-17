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
	"encoding/json"
	"time"

	redis "github.com/go-redis/redis/v8"
)

// RedisCache redis cache
type RedisCache struct {
	client    redis.UniversalClient
	ttl       time.Duration
	prefix    string
	unmarshal func(data []byte, v interface{}) error
	marshal   func(v interface{}) ([]byte, error)
}

// RedisCacheOption redis cache option
type RedisCacheOption func(c *RedisCache)

// Done done function
type Done func() error

// noop noop
var noop = func() error {
	return nil
}

type ttlData struct {
	ExpiredAt time.Time   `json:"expiredAt"`
	Data      interface{} `json:"data"`
}

// NewRedisCache returns a new redis cache
func NewRedisCache(c redis.UniversalClient, opts ...RedisCacheOption) *RedisCache {
	rc := &RedisCache{
		client: c,
	}
	for _, opt := range opts {
		opt(rc)
	}
	return rc
}

func newCompressRedisCache(c redis.UniversalClient, compressor *compressor, opts ...RedisCacheOption) *RedisCache {
	opts = append([]RedisCacheOption{
		RedisCacheMarshalOption(compressor.Marshal),
		RedisCacheUnmarshalOption(compressor.Unmarshal),
	}, opts...)
	return NewRedisCache(c, opts...)
}

// NewSnappyRedisCache returns a redis cache with snappy compressor
func NewSnappyRedisCache(c redis.UniversalClient, compressMinLength int, opts ...RedisCacheOption) *RedisCache {
	if compressMinLength <= 0 {
		panic("compress mini length should be gt 0")
	}
	compressor := NewSnappyCompressor(compressMinLength)
	return newCompressRedisCache(c, compressor, opts...)
}

// NewZSTDRedisCache returns a redis cache with zstd compressor
func NewZSTDRedisCache(c redis.UniversalClient, compressMinLength int, opts ...RedisCacheOption) *RedisCache {
	if compressMinLength <= 0 {
		panic("compress mini length should be gt 0")
	}
	compressor := NewZSTDCompressor(compressMinLength)
	return newCompressRedisCache(c, compressor, opts...)
}

// NewLZ4RedisCache returns a redis cache with lz4 compressor
func NewLZ4RedisCache(c redis.UniversalClient, compressMinLength int, opts ...RedisCacheOption) *RedisCache {
	if compressMinLength <= 0 {
		panic("compress mini length should be gt 0")
	}
	compressor := NewLZ4Compressor(compressMinLength)
	return newCompressRedisCache(c, compressor, opts...)
}

// NewCompressRedisCache the same as NewSnappyRedisCache
func NewCompressRedisCache(c redis.UniversalClient, compressMinLength int, opts ...RedisCacheOption) *RedisCache {
	return NewSnappyRedisCache(c, compressMinLength, opts...)
}

// RedisCacheTTLOption redis cache ttl option
func RedisCacheTTLOption(ttl time.Duration) RedisCacheOption {
	return func(c *RedisCache) {
		c.ttl = ttl
	}
}

// RedisCachePrefixOption redis cache prefix option
func RedisCachePrefixOption(prefix string) RedisCacheOption {
	return func(c *RedisCache) {
		c.prefix = prefix
	}
}

// RedisCacheUnmarshalOption redis cache unmarshal option
func RedisCacheUnmarshalOption(fn func(data []byte, v interface{}) error) RedisCacheOption {
	return func(c *RedisCache) {
		c.unmarshal = fn
	}
}

// RedisCacheMarshalOption redis cache marshal option
func RedisCacheMarshalOption(fn func(v interface{}) ([]byte, error)) RedisCacheOption {
	return func(c *RedisCache) {
		c.marshal = fn
	}
}

// getTTL gets ttl of cache
func (c *RedisCache) getTTL(ttl ...time.Duration) time.Duration {
	value := c.ttl
	if len(ttl) != 0 {
		value = ttl[0]
	}
	if value != 0 {
		return value
	}
	return defaultRedisTTL
}

// getKey gets key for cache, prefix + key
func (c *RedisCache) getKey(key string) (string, error) {
	if key == "" {
		return "", ErrKeyIsNil
	}
	return c.prefix + key, nil
}

func (c *RedisCache) lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, true, ttl).Result()
}

// Lock the key for ttl, ii will return true, nil if success
func (c *RedisCache) Lock(ctx context.Context, key string, ttl ...time.Duration) (bool, error) {
	key, err := c.getKey(key)
	if err != nil {
		return false, err
	}
	d := c.getTTL(ttl...)
	return c.lock(ctx, key, d)
}

func (c *RedisCache) del(ctx context.Context, key string) (int64, error) {
	// Key在public的方法中已完成添加前缀，因此不需要再添加
	return c.client.Del(ctx, key).Result()
}

// Del deletes data from cache
func (c *RedisCache) Del(ctx context.Context, key string) (int64, error) {
	key, err := c.getKey(key)
	if err != nil {
		return 0, err
	}
	return c.del(ctx, key)
}

// LockWithDone locks the key for ttl and return done function to delete the lock
func (c *RedisCache) LockWithDone(ctx context.Context, key string, ttl ...time.Duration) (bool, Done, error) {
	key, err := c.getKey(key)
	if err != nil {
		return false, noop, err
	}
	d := c.getTTL(ttl...)
	success, err := c.lock(ctx, key, d)
	// 如果lock失败，则返回no op 的done function
	if err != nil || !success {
		return false, noop, err
	}
	done := func() error {
		_, err := c.del(ctx, key)
		return err
	}
	return true, done, nil
}

func (c *RedisCache) txPipeline() redis.Pipeliner {
	return c.client.TxPipeline()
}

// IncWith inc the value of key from cache
func (c *RedisCache) IncWith(ctx context.Context, key string, value int64, ttl ...time.Duration) (int64, error) {
	key, err := c.getKey(key)
	if err != nil {
		return 0, err
	}
	pipe := c.txPipeline()
	// 保证只有首次会设置ttl
	d := c.getTTL(ttl...)
	pipe.SetNX(ctx, key, 0, d)
	incr := pipe.IncrBy(ctx, key, value)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	count := incr.Val()
	return count, nil
}

// Get gets value from cache
func (c *RedisCache) get(ctx context.Context, key string) ([]byte, error) {
	// 避免多次调用getKey，
	// 由public的方法来处理getkey，因此不再需要调用getKey
	return c.client.Get(ctx, key).Bytes()
}

// Get gets value from cache
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, err
	}
	return c.get(ctx, key)
}

// GetIgnoreNilErr gets value from cache and ignore nil err
func (c *RedisCache) GetIgnoreNilErr(ctx context.Context, key string) ([]byte, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, err
	}
	result, err := c.get(ctx, key)
	if err != nil && err != redis.Nil {
		return nil, err
	}
	return result, nil
}

// GetAndDel gets value and deletes it remove cache
func (c *RedisCache) GetAndDel(ctx context.Context, key string) ([]byte, error) {
	key, err := c.getKey(key)
	if err != nil {
		return nil, err
	}
	pipe := c.txPipeline()
	cmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	return cmd.Bytes()
}

func (c *RedisCache) set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Set sets data to cache, if ttl is not nil, it will use default ttl
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	d := c.getTTL(ttl...)
	return c.set(ctx, key, value, d)
}

func (c *RedisCache) doUnmarshal(data []byte, value interface{}) error {
	if c.unmarshal != nil {
		return c.unmarshal(data, value)
	}
	return json.Unmarshal(data, value)
}

func (c *RedisCache) doMarshal(value interface{}) ([]byte, error) {
	if c.marshal != nil {
		return c.marshal(value)
	}
	return json.Marshal(value)
}

func (c *RedisCache) getStruct(ctx context.Context, key string, value interface{}) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}

	result, err := c.get(ctx, key)
	if err != nil {
		return err
	}
	return c.doUnmarshal(result, value)
}

// GetStruct gets cache and unmarshal to struct
func (c *RedisCache) GetStruct(ctx context.Context, key string, value interface{}) error {
	return c.getStruct(ctx, key, value)
}

// GetStructAndTTL gets cache, unmarshal to struct and return the ttl of it. The cache should be set by SetStructWithTTL
func (c *RedisCache) GetStructAndTTL(ctx context.Context, key string, value interface{}) (time.Duration, error) {
	data := ttlData{
		Data: value,
	}
	err := c.getStruct(ctx, key, &data)
	if err != nil {
		return 0, err
	}
	// 如果无设置expired at
	if data.ExpiredAt.IsZero() {
		return -1, nil
	}
	return time.Until(data.ExpiredAt), nil
}

// GetStructWithDone gets data from redis and unmarshal to struct,
// it returns a done function to delete the data.
func (c *RedisCache) GetStructWithDone(ctx context.Context, key string, value interface{}) (Done, error) {
	err := c.getStruct(ctx, key, value)
	if err != nil {
		return noop, err
	}
	return func() error {
		_, err := c.Del(ctx, key)
		return err
	}, nil
}

func (c *RedisCache) setStruct(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	key, err := c.getKey(key)
	if err != nil {
		return err
	}
	buf, err := c.doMarshal(value)
	if err != nil {
		return err
	}
	d := c.getTTL(ttl...)
	return c.set(ctx, key, buf, d)
}

// SetStruct marshals struct to bytes and sets to cache, if it will use default ttl if ttl is nil
func (c *RedisCache) SetStruct(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	return c.setStruct(ctx, key, value, ttl...)
}

// SetStructWithTTL adds expiredAt field to data, marshals data to bytes and sets to cache
func (c *RedisCache) SetStructWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data := ttlData{
		ExpiredAt: time.Now().Add(ttl),
		Data:      value,
	}
	return c.setStruct(ctx, key, data, ttl)
}

// TTL returns the ttl of the key
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	key, err := c.getKey(key)
	if err != nil {
		return 0, err
	}
	return c.client.TTL(ctx, key).Result()
}
