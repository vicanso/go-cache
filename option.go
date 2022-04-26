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
	cleanWindow      time.Duration
	maxEntrySize     int
	hardMaxCacheSize int
	compressor       Compressor
	onRemove         func(key string)
}

// CacheOption cache option
type CacheOption func(opt *Option)

func CacheCleanWindowOption(cleanWindow time.Duration) CacheOption {
	return func(opt *Option) {
		opt.cleanWindow = cleanWindow
	}
}

func CacheMaxEntrySizeOption(maxEntrySize int) CacheOption {
	return func(opt *Option) {
		opt.maxEntrySize = maxEntrySize
	}
}

func CacheHardMaxCacheSizeOption(hardMaxCacheSize int) CacheOption {
	return func(opt *Option) {
		opt.hardMaxCacheSize = hardMaxCacheSize
	}
}

func CacheOnRemoveOption(onRemove func(key string)) CacheOption {
	return func(opt *Option) {
		opt.onRemove = onRemove
	}
}

func CacheKeyPrefixOption(keyPrefix string) CacheOption {
	return func(opt *Option) {
		opt.keyPrefix = keyPrefix
	}
}

func CacheStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.store = store
	}
}

func CacheSecondaryStoreOption(store Store) CacheOption {
	return func(opt *Option) {
		opt.secondaryStore = store
	}
}

func CacheCompressorOption(compressor Compressor) CacheOption {
	return func(opt *Option) {
		opt.compressor = compressor
	}
}

func CacheSnappyOption(minCompressLength int) CacheOption {
	return func(opt *Option) {
		opt.compressor = NewSnappyCompressor(minCompressLength)
	}
}

func CacheZSTDOption(minCompressLength, level int) CacheOption {
	return func(opt *Option) {
		opt.compressor = NewZSTDCompressor(minCompressLength, level)
	}
}
