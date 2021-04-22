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

	"github.com/go-redis/redis/v8"
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
func (c *RedisCache) getKey(key string) string {
	return c.prefix + key
}

func (c *RedisCache) lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, true, ttl).Result()
}

// Lock the key for ttl, ii will return true, nil if success
func (c *RedisCache) Lock(ctx context.Context, key string, ttl ...time.Duration) (bool, error) {
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.lock(ctx, key, d)
}

func (c *RedisCache) del(ctx context.Context, key string) (count int64, err error) {
	// Key在public的方法中已完成添加前缀，因此不需要再添加
	return c.client.Del(ctx, key).Result()
}

// Del deletes data from cache
func (c *RedisCache) Del(ctx context.Context, key string) (count int64, err error) {
	return c.del(ctx, c.getKey(key))
}

// LockWithDone locks the key for ttl and return done function to delete the lock
func (c *RedisCache) LockWithDone(ctx context.Context, key string, ttl ...time.Duration) (bool, Done, error) {
	key = c.getKey(key)
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
func (c *RedisCache) IncWith(ctx context.Context, key string, value int64, ttl ...time.Duration) (count int64, err error) {
	key = c.getKey(key)
	pipe := c.txPipeline()
	// 保证只有首次会设置ttl
	d := c.getTTL(ttl...)
	pipe.SetNX(ctx, key, 0, d)
	var incr *redis.IntCmd
	if value < 1 {
		value = 1
	}
	incr = pipe.IncrBy(ctx, key, value)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return
	}
	count = incr.Val()
	return
}

// Get gets value from cache
func (c *RedisCache) get(ctx context.Context, key string) (result []byte, err error) {
	// 避免多次调用getKey，
	// 由public的方法来处理getkey，因此不再需要调用getKey
	return c.client.Get(ctx, key).Bytes()
}

// Get gets value from cache
func (c *RedisCache) Get(ctx context.Context, key string) (result []byte, err error) {
	return c.get(ctx, c.getKey(key))
}

// GetIgnoreNilErr gets value from cache and ignore nil err
func (c *RedisCache) GetIgnoreNilErr(ctx context.Context, key string) (result []byte, err error) {
	result, err = c.get(ctx, c.getKey(key))
	if err == redis.Nil {
		err = nil
	}
	return
}

// GetAndDel gets value and deletes it remove cache
func (c *RedisCache) GetAndDel(ctx context.Context, key string) (result []byte, err error) {
	pipe := c.txPipeline()
	key = c.getKey(key)
	cmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return
	}
	return cmd.Bytes()
}

func (c *RedisCache) set(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error) {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Set sets data to cache, if ttl is not nil, it will use default ttl
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) (err error) {
	key = c.getKey(key)
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

// GetStruct gets cache and unmarshal to struct
func (c *RedisCache) GetStruct(ctx context.Context, key string, value interface{}) (err error) {
	result, err := c.get(ctx, c.getKey(key))
	if err != nil {
		return
	}
	return c.doUnmarshal(result, value)
}

// GetStructWithDone gets data from redis and unmarshal to struct,
// it returns a done function to delete the data.
func (c *RedisCache) GetStructWithDone(ctx context.Context, key string, value interface{}) (done Done, err error) {
	err = c.GetStruct(ctx, key, value)
	if err != nil {
		done = noop
		return
	}
	return func() error {
		_, err := c.Del(context.Background(), key)
		return err
	}, nil
}

// SetStruct marshals struct to bytes and sets to cache, if it will use default ttl if ttl is nil
func (c *RedisCache) SetStruct(ctx context.Context, key string, value interface{}, ttl ...time.Duration) (err error) {
	buf, err := c.doMarshal(value)
	if err != nil {
		return
	}
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.set(ctx, key, buf, d)
}
