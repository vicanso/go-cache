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
	"time"
)

type Option struct {
	store            Store
	secondaryStore   Store
	keyPrefix        string
	ttlList          []time.Duration
	cleanWindow      time.Duration
	maxEntrySize     int
	hardMaxCacheSize int
	shards           int
	compressor       Compressor
	onRemove         func(key string)
}

// CacheOption cache option
type CacheOption func(opt *Option)

// CacheCleanWindowOption set clean window for bigcache store
func CacheCleanWindowOption(cleanWindow time.Duration) CacheOption {
	return func(opt *Option) {
		opt.cleanWindow = cleanWindow
	}
}

// CacheMaxEntrySizeOption set max entry size for bigcache store
func CacheMaxEntrySizeOption(maxEntrySize int) CacheOption {
	return func(opt *Option) {
		opt.maxEntrySize = maxEntrySize
	}
}

// CacheHardMaxCacheSizeOption set hard max cache size for bigcache store
func CacheHardMaxCacheSizeOption(hardMaxCacheSize int) CacheOption {
	return func(opt *Option) {
		opt.hardMaxCacheSize = hardMaxCacheSize
	}
}

// CacheShardsOption set shards for bigcache store
func CacheShardsOption(shards int) CacheOption {
	return func(opt *Option) {
		opt.shards = shards
	}
}

// CacheOnRemoveOption set on remove function for bigcache store
func CacheOnRemoveOption(onRemove func(key string)) CacheOption {
	return func(opt *Option) {
		opt.onRemove = onRemove
	}
}

// CacheKeyPrefixOption set key prefix for store
func CacheKeyPrefixOption(keyPrefix string) CacheOption {
	return func(opt *Option) {
		opt.keyPrefix = keyPrefix
	}
}

// CacheStoreOption set custom store for cache, the bigcache store will be used as default
func CacheStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.store = store
	}
}

// CacheSecondaryStoreOption set secondary store for cache, it is slower with greater capacity
func CacheSecondaryStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.secondaryStore = store
	}
}

// CacheCompressorOption set compressor for store, the data will be compressed if matched
func CacheCompressorOption(compressor Compressor) CacheOption {
	return func(opt *Option) {
		opt.compressor = compressor
	}
}

// CacheSnappyOption set snappy compress for store
func CacheSnappyOption(minCompressLength int) CacheOption {
	return CacheCompressorOption(NewSnappyCompressor(minCompressLength))
}

// CacheZSTDOption set zstd compress for store
func CacheZSTDOption(minCompressLength, level int) CacheOption {
	return CacheCompressorOption(NewZSTDCompressor(minCompressLength, level))
}

// CacheMultiTTLOption set multi ttl for store
func CacheMultiTTLOption(ttlList []time.Duration) CacheOption {
	return func(opt *Option) {
		opt.ttlList = ttlList
	}
}
