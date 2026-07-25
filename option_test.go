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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheOption(t *testing.T) {
	assert := assert.New(t)
	store, err := NewBigCacheStore(time.Minute)
	assert.Nil(err)
	secondary, err := NewBigCacheStore(time.Minute)
	assert.Nil(err)

	fns := []CacheOption{
		CacheKeyPrefixOption("prefix"),
		CacheZSTDOption(10, 1),
		CacheStoreOption(store),
		CacheSecondaryStoreOption(secondary),
		CacheMultiTTLOption([]time.Duration{
			time.Second,
			2 * time.Second,
		}),
	}
	opt := Option{}
	for _, fn := range fns {
		fn(&opt)
	}
	assert.Equal("prefix", opt.keyPrefix)
	assert.NotNil(opt.compressor)
	assert.Equal(store, opt.store)
	assert.Equal(secondary, opt.secondaryStore)
	assert.Equal([]time.Duration{
		time.Second,
		2 * time.Second,
	}, opt.ttlList)
}

func TestBigCacheOption(t *testing.T) {
	assert := assert.New(t)

	fns := []BigCacheOption{
		BigCacheCleanWindowOption(time.Minute),
		BigCacheMaxEntrySizeOption(512),
		BigCacheHardMaxCacheSizeOption(1024 * 1024),
		BigCacheShardsOption(16),
		BigCacheOnRemoveOption(func(key string) {
		}),
	}
	opt := bigCacheOptions{}
	for _, fn := range fns {
		fn(&opt)
	}
	assert.Equal(time.Minute, opt.cleanWindow)
	assert.Equal(512, opt.maxEntrySize)
	assert.Equal(1024*1024, opt.hardMaxCacheSize)
	assert.Equal(16, opt.shards)
	assert.NotNil(opt.onRemove)
}
