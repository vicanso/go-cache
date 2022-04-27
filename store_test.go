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
				return newBigCacheStore(time.Minute, &Option{
					cleanWindow:      10 * time.Second,
					maxEntrySize:     1000,
					hardMaxCacheSize: 10 * 1024 * 1024,
					onRemove: func(key string) {

					},
				})
			},
		},
		{
			newStore: func() (Store, error) {
				return NewRedisStore(newClient()), nil
			},
		},
	}

	for _, tt := range tests {
		store, err := tt.newStore()
		assert.Nil(err)
		defer store.Close(context.Background())

		key := randomString()
		value := []byte("value")
		_, err = store.Get(context.Background(), key)
		assert.Equal(ErrIsNil, err)

		err = store.Set(context.Background(), key, value, time.Second)
		assert.Nil(err)

		v, err := store.Get(context.Background(), key)
		assert.Nil(err)
		assert.Equal(value, v)

		err = store.Delete(context.Background(), key)
		assert.Nil(err)

		_, err = store.Get(context.Background(), key)
		assert.Equal(ErrIsNil, err)
	}
}
