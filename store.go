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
)

// Store interface for cache
type Store interface {
	// Set sets data to store. Callers (Cache) clone per store before Set, but
	// implementations that retain the slice asynchronously should still copy.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Get gets data from store. A miss should return ErrNotFound (or ErrIsNil).
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete deletes data from store
	Delete(ctx context.Context, key string) error
	// Close closes the store. Must not close shared external clients unless
	// the store was constructed to own them (see NewRedisStoreOwned).
	Close(ctx context.Context) error
}
