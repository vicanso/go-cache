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

func (rs *RedisSession) getKey(key string) (string, error) {
	if key == "" {
		return "", ErrKeyIsNil
	}
	return rs.prefix + key, nil
}

// SetPrefix sets prefix for redis session's key
func (rs *RedisSession) SetPrefix(prefix string) {
	rs.prefix = prefix
}

// Get session from redis, it will not return error if data is not exists
func (rs *RedisSession) Get(ctx context.Context, key string) ([]byte, error) {
	key, err := rs.getKey(key)
	if err != nil {
		return nil, err
	}
	result, err := rs.client.Get(ctx, key).Bytes()
	// 如果查询失败，返回空，redis session针对获取不到的不需要直接返回出错
	if err == redis.Nil {
		err = nil
	}
	return result, err
}

// Set session to redis
func (rs *RedisSession) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	key, err := rs.getKey(key)
	if err != nil {
		return err
	}
	return rs.client.Set(ctx, key, data, ttl).Err()
}

// Destroy session from redis
func (rs *RedisSession) Destroy(ctx context.Context, key string) error {
	key, err := rs.getKey(key)
	if err != nil {
		return err
	}
	return rs.client.Del(ctx, key).Err()
}
