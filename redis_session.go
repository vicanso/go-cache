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

	"github.com/go-redis/redis/v8"
)

type redisSession struct {
	client  *redis.Client
	cluster *redis.ClusterClient
	prefix  string
}

// NewRedisSession create a new redis session
func NewRedisSession(c *redis.Client) *redisSession {
	return &redisSession{
		client: c,
	}
}

// NewRedisClusterSession create a new redis cluster session
func NewRedisClusterSession(c *redis.ClusterClient) *redisSession {
	return &redisSession{
		cluster: c,
	}
}

func (rs *redisSession) getKey(key string) string {
	return rs.prefix + key
}

// SetPrefix set prefix for redis session
func (rs *redisSession) SetPrefix(prefix string) {
	rs.prefix = prefix
}

// Get get session for redis
func (rs *redisSession) Get(key string) (result []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	if rs.cluster != nil {
		result, err = rs.cluster.Get(ctx, key).Bytes()
	} else {
		result, err = rs.client.Get(ctx, key).Bytes()
	}
	// 如果查询失败，返回空，redis session针对获取不到的不需要直接返回出错
	if err == redis.Nil {
		err = nil
	}
	return
}

// Set set session to redis
func (rs *redisSession) Set(key string, data []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	if rs.cluster != nil {
		return rs.cluster.Set(ctx, key, data, ttl).Err()
	}
	return rs.client.Set(ctx, key, data, ttl).Err()
}

// Destroy destroy session from redis
func (rs *redisSession) Destroy(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTTL)
	defer cancel()
	key = rs.getKey(key)
	if rs.cluster != nil {
		return rs.cluster.Del(ctx, key).Err()
	}
	return rs.client.Del(ctx, key).Err()
}
