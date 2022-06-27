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
	// Set sets data to store, the value should be copy before save to store
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Get gets data from store
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete deletes data form store
	Delete(ctx context.Context, key string) error
	// Close closes the store
	Close(ctx context.Context) error
}
