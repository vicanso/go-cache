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

	redis "github.com/go-redis/redis/v8"
)

type RedisSession struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisSession returns a new redis session
func NewRedisSession(c redis.UniversalClient) *RedisSession {
	return &RedisSession{
		client: c,
	}
}

func (rs *RedisSession) getKey(key string) string {
	return rs.prefix + key
}

// SetPrefix sets prefix for redis session's key
func (rs *RedisSession) SetPrefix(prefix string) {
	rs.prefix = prefix
}

// Get session from redis, it will not return error if data is not exists
func (rs *RedisSession) Get(key string) (result []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	result, err = rs.client.Get(ctx, key).Bytes()
	// 如果查询失败，返回空，redis session针对获取不到的不需要直接返回出错
	if err == redis.Nil {
		err = nil
	}
	return
}

// Set session to redis
func (rs *RedisSession) Set(key string, data []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	return rs.client.Set(ctx, key, data, ttl).Err()
}

// Destroy session from redis
func (rs *RedisSession) Destroy(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	return rs.client.Del(ctx, key).Err()
}
