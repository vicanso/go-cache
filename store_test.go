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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		newStore func() (Store, error)
	}{
		{
			newStore: func() (Store, error) {
				return NewBigCacheStore(
					time.Minute,
					BigCacheCleanWindowOption(10*time.Second),
					BigCacheMaxEntrySizeOption(1000),
					BigCacheHardMaxCacheSizeOption(10*1024*1024),
					BigCacheShardsOption(4),
					BigCacheOnRemoveOption(func(key string) {

					}),
				)
			},
		},
		{
			newStore: func() (Store, error) {
				c, _ := newClient(t)
				return NewRedisStore(c), nil
			},
		},
	}

	for _, tt := range tests {
		store, err := tt.newStore()
		assert.Nil(err)
		defer store.Close(context.Background()) //nolint:errcheck

		key := randomString()
		value := []byte("value")
		_, err = store.Get(context.Background(), key)
		assert.True(errors.Is(err, ErrNotFound))

		err = store.Set(context.Background(), key, value, time.Second)
		assert.Nil(err)

		v, err := store.Get(context.Background(), key)
		assert.Nil(err)
		assert.Equal(value, v)

		err = store.Delete(context.Background(), key)
		assert.Nil(err)

		_, err = store.Get(context.Background(), key)
		assert.True(errors.Is(err, ErrNotFound))
	}
}

func TestRedisStoreCloseOwnership(t *testing.T) {
	assert := assert.New(t)
	c, _ := newClient(t)

	// Default store must not close the shared client
	store := NewRedisStore(c)
	assert.Nil(store.Close(context.Background()))
	assert.Nil(c.Ping(context.Background()).Err())

	// Owned store closes the client
	c2, _ := newClient(t)
	owned := NewRedisStoreOwned(c2)
	assert.Nil(owned.Close(context.Background()))
	err := c2.Ping(context.Background()).Err()
	assert.NotNil(err)
}
