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
	client redis.UniversalClient
}

func (rs *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return rs.client.Set(ctx, key, value, ttl).Err()
}

func (rs *redisStore) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := rs.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		err = ErrIsNil
	}
	return data, err
}

func (rs *redisStore) Close(_ context.Context) error {
	return rs.client.Close()
}

func (rs *redisStore) Delete(ctx context.Context, key string) error {
	return rs.client.Del(ctx, key).Err()
}

func NewRedisStore(client redis.UniversalClient) Store {
	return &redisStore{
		client: client,
	}
}
