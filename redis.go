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
	"github.com/golang/snappy"
)

// redisCache redis cache
type redisCache struct {
	client    *redis.Client
	cluster   *redis.ClusterClient
	ttl       time.Duration
	prefix    string
	unmarshal func(data []byte, v interface{}) error
	marshal   func(v interface{}) ([]byte, error)
}

// Done done function
type Done func() error

// noop noop
var noop = func() error {
	return nil
}

func snappyEncode(data []byte) []byte {
	dst := []byte{}
	dst = snappy.Encode(dst, data)
	return dst
}

func snappyDecode(buf []byte) ([]byte, error) {
	var dst []byte
	return snappy.Decode(dst, buf)
}

// NewRedisCache create a new redis cache
func NewRedisCache(c *redis.Client) *redisCache {
	return &redisCache{
		client: c,
	}
}

// NewRedisClusterCache create a new redis cluster cache
func NewRedisClusterCache(c *redis.ClusterClient) *redisCache {
	return &redisCache{
		cluster: c,
	}
}

// SetTTL set default ttl for cache
func (c *redisCache) SetTTL(ttl time.Duration) *redisCache {
	c.ttl = ttl
	return c
}

// getTTL get ttl
func (c *redisCache) getTTL(ttl ...time.Duration) time.Duration {
	value := c.ttl
	if len(ttl) != 0 {
		value = ttl[0]
	}
	if value != 0 {
		return value
	}
	return defaultRedisTTL
}

// SetPrefix set prefix for cache
func (c *redisCache) SetPrefix(prefix string) *redisCache {
	c.prefix = prefix
	return c
}

// getKey get key for cache, prefix + key
func (c *redisCache) getKey(key string) string {
	return c.prefix + key
}

// SetUnmarshal set unmarshal function
func (c *redisCache) SetUnmarshal(fn func(data []byte, v interface{}) error) {
	c.unmarshal = fn
}

// SetMarshal set marshal function
func (c *redisCache) SetMarshal(fn func(v interface{}) ([]byte, error)) {
	c.marshal = fn
}

func (c *redisCache) lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if c.cluster != nil {
		return c.cluster.SetNX(ctx, key, true, ttl).Result()
	}
	return c.client.SetNX(ctx, key, true, ttl).Result()
}

// Lock lock the key for ttl
func (c *redisCache) Lock(ctx context.Context, key string, ttl ...time.Duration) (bool, error) {
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.lock(ctx, key, d)
}

func (c *redisCache) del(ctx context.Context, key string) (count int64, err error) {
	// key = c.getKey(key)
	if c.cluster != nil {
		return c.cluster.Del(ctx, key).Result()
	}
	return c.client.Del(ctx, key).Result()
}

// Del delete cache
func (c *redisCache) Del(ctx context.Context, key string) (count int64, err error) {
	return c.del(ctx, c.getKey(key))
}

// LockWithDone lock the key for ttl and return done function to delete
func (c *redisCache) LockWithDone(ctx context.Context, key string, ttl ...time.Duration) (bool, Done, error) {
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

func (c *redisCache) txPipeline() redis.Pipeliner {
	if c.cluster != nil {
		return c.cluster.TxPipeline()
	}
	return c.client.TxPipeline()
}

// IncWith inc the cache
func (c *redisCache) IncWith(ctx context.Context, key string, value int64, ttl ...time.Duration) (count int64, err error) {
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

// Get get value from cache
func (c *redisCache) get(ctx context.Context, key string) (result []byte, err error) {
	// key = c.getKey(key)
	if c.cluster != nil {
		return c.cluster.Get(ctx, key).Bytes()
	}
	return c.client.Get(ctx, key).Bytes()
}

// Get get value from cache
func (c *redisCache) Get(ctx context.Context, key string) (result []byte, err error) {
	return c.get(ctx, c.getKey(key))
}

// GetIgnoreNilErr get value from cache and ignore nil err
func (c *redisCache) GetIgnoreNilErr(ctx context.Context, key string) (result []byte, err error) {
	result, err = c.get(ctx, c.getKey(key))
	if err == redis.Nil {
		err = nil
	}
	return
}

// GetAndDel get value and delete it remove cache
func (c *redisCache) GetAndDel(ctx context.Context, key string) (result []byte, err error) {
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

func (c *redisCache) set(ctx context.Context, key string, value interface{}, ttl time.Duration) (err error) {
	if c.cluster != nil {
		return c.cluster.Set(ctx, key, value, ttl).Err()
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Set set cache
func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) (err error) {
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.set(ctx, key, value, d)
}

func (c *redisCache) doUnmarshal(data []byte, value interface{}) error {
	if c.unmarshal != nil {
		return c.unmarshal(data, value)
	}
	return json.Unmarshal(data, value)
}

func (c *redisCache) doMarshal(value interface{}) ([]byte, error) {
	if c.marshal != nil {
		return c.marshal(value)
	}
	return json.Marshal(value)
}

// GetStruct get cache and unmarshal to struct
func (c *redisCache) GetStruct(ctx context.Context, key string, value interface{}) (err error) {
	result, err := c.get(ctx, c.getKey(key))
	if err != nil {
		return
	}
	return c.doUnmarshal(result, value)
}

// SetStruct set struct to cache
func (c *redisCache) SetStruct(ctx context.Context, key string, value interface{}, ttl ...time.Duration) (err error) {
	buf, err := c.doMarshal(value)
	if err != nil {
		return
	}
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.set(ctx, key, buf, d)
}

// GetStructSnappy get struct from cache
func (c *redisCache) GetStructSnappy(ctx context.Context, key string, value interface{}) (err error) {
	result, err := c.get(ctx, c.getKey(key))
	if err != nil {
		return
	}
	result, err = snappyDecode(result)
	if err != nil {
		return
	}
	return c.doUnmarshal(result, value)
}

// SetStructSnappy set struct to cache, it recommend for data gt 10KB
func (c *redisCache) SetStructSnappy(ctx context.Context, key string, value interface{}, ttl ...time.Duration) (err error) {
	buf, err := c.doMarshal(value)
	if err != nil {
		return
	}
	buf = snappyEncode(buf)
	key = c.getKey(key)
	d := c.getTTL(ttl...)
	return c.set(ctx, key, buf, d)
}
