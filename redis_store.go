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
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client      redis.UniversalClient
	closeClient bool
}

func (rs *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return rs.client.Set(ctx, key, value, ttl).Err()
}

func (rs *redisStore) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := rs.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		err = ErrNotFound
	}
	return data, err
}

// Close closes the underlying client only when the store owns it
// (created via NewRedisStoreOwned). NewRedisStore never closes a shared client.
func (rs *redisStore) Close(_ context.Context) error {
	if !rs.closeClient {
		return nil
	}
	return rs.client.Close()
}

func (rs *redisStore) Delete(ctx context.Context, key string) error {
	return rs.client.Del(ctx, key).Err()
}

// NewRedisStore wraps a redis client as a Store. The client is not closed by
// Store.Close — the caller retains ownership (typical when the client is shared).
func NewRedisStore(client redis.UniversalClient) Store {
	return &redisStore{
		client: client,
	}
}

// NewRedisStoreOwned wraps a redis client as a Store that closes the client
// when Store.Close is called. Use only when the cache exclusively owns the client.
func NewRedisStoreOwned(client redis.UniversalClient) Store {
	return &redisStore{
		client:      client,
		closeClient: true,
	}
}
