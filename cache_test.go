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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testData struct {
	Name string `json:"name"`
}

type testDataCustom testData

func (t *testDataCustom) Marshal(v any) ([]byte, error) {
	return []byte(t.Name), nil
}

func (t *testDataCustom) Unmarshal(data []byte, v any) error {
	t.Name = string(data)
	return nil
}

func TestCacheGetKey(t *testing.T) {
	assert := assert.New(t)

	c, err := New(
		1*time.Second,
		CacheKeyPrefixOption("prefix:"),
	)
	assert.Nil(err)

	_, err = c.getKey("")
	assert.Equal(ErrKeyIsNil, err)
	key, err := c.getKey("a")
	assert.Nil(err)
	assert.Equal("prefix:a", key)
}

func TestCacheGetSet(t *testing.T) {
	assert := assert.New(t)

	c, err := New(100 * time.Millisecond)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := "key"
	err = c.Set(context.Background(), key, &testData{
		Name: "test data",
	})
	assert.Nil(err)
	data := testData{}
	err = c.Get(context.Background(), key, &data)
	assert.Nil(err)
	assert.Equal("test data", data.Name)

	// custom marshal/unmarshal
	keyCustom := "keyCustom"
	err = c.Set(context.Background(), keyCustom, &testDataCustom{
		Name: "test data custom",
	})
	assert.Nil(err)
	dataCustom := testDataCustom{}
	err = c.Get(context.Background(), keyCustom, &dataCustom)
	assert.Nil(err)
	assert.Equal("test data custom", dataCustom.Name)

	result, err := Get[testDataCustom](context.Background(), c, keyCustom)
	assert.Nil(err)
	assert.Equal("test data custom", result.Name)

	err = c.Get(context.Background(), "abc", nil)
	assert.Equal(ErrIsNil, err)

	// 逻辑过期由内嵌时间戳判定，无需等待秒级
	time.Sleep(200 * time.Millisecond)
	err = c.Get(context.Background(), keyCustom, nil)
	assert.Equal(ErrIsNil, err)
}

func TestCacheCompress(t *testing.T) {
	assert := assert.New(t)

	store, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)
	c, err := New(
		time.Minute,
		CacheStoreOption(store),
		CacheSnappyOption(1),
	)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := "key"
	value := []byte("Hello World!Hello World!Hello World!")
	err = c.SetBytes(context.Background(), key, value)
	assert.Nil(err)

	buf, err := store.Get(context.Background(), key)
	assert.Nil(err)
	assert.True(len(buf) < len(value))

	buf, err = c.GetBytes(context.Background(), key)
	assert.Nil(err)
	assert.Equal(value, buf)
}

func TestCacheGetTTL(t *testing.T) {
	assert := assert.New(t)
	c := Cache{
		ttlList: []time.Duration{
			time.Second,
			2 * time.Second,
		},
	}
	assert.Equal(3*time.Second, c.getTTL(0, 3*time.Second))
	assert.Equal(time.Second, c.getTTL(0))
	assert.Equal(2*time.Second, c.getTTL(1))
}

func TestCacheMultiStore(t *testing.T) {
	assert := assert.New(t)

	s1, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)
	s2, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)

	c, err := New(
		time.Minute,
		CacheStoreOption(s1),
		CacheSecondaryStoreOption(s2),
	)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := "key"
	value := []byte("value")
	err = c.SetBytes(context.Background(), key, value)
	assert.Nil(err)
	buf, err := s1.Get(context.Background(), key)
	assert.Nil(err)
	assert.Equal(value, buf[timestampByteSize:])
	buf, err = s2.Get(context.Background(), key)
	assert.Nil(err)
	assert.Equal(value, buf[timestampByteSize:])

	// 一级缓存清除
	err = s1.Delete(context.Background(), key)
	assert.Nil(err)
	_, err = s1.Get(context.Background(), key)
	assert.Equal(ErrIsNil, err)
	// 获取时会重新更新一级缓存
	buf, err = c.GetBytes(context.Background(), key)
	assert.Nil(err)
	assert.NotEmpty(buf)
	_, err = s1.Get(context.Background(), key)
	assert.Nil(err)

	err = c.Delete(context.Background(), key)
	assert.Nil(err)
}

func TestCacheMultiTTL(t *testing.T) {
	assert := assert.New(t)
	s1, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)
	s2, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)

	c, err := New(
		time.Minute,
		CacheStoreOption(s1),
		CacheSecondaryStoreOption(s2),
		CacheMultiTTLOption([]time.Duration{
			100 * time.Millisecond,
			10 * time.Minute,
		}),
	)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := "key"
	value := []byte("value")
	err = c.SetBytes(context.Background(), key, value)
	assert.Nil(err)
	result, ttl, err := c.GetBytesAndTTL(context.Background(), key)
	assert.Nil(err)
	assert.Equal(value, result)
	assert.NotEmpty(ttl)
	// 一级缓存逻辑过期后应回落到二级缓存并回填
	time.Sleep(200 * time.Millisecond)
	_, ttl, err = c.GetBytesAndTTL(context.Background(), key)
	assert.Nil(err)
	assert.Equal(value, result)
	assert.NotEmpty(ttl)
}

func TestGetAndTTL(t *testing.T) {
	assert := assert.New(t)
	c, err := New(time.Minute)
	assert.Nil(err)

	key := "key"
	data := map[string]string{
		"a": "1",
	}
	err = c.Set(context.Background(), key, data)
	assert.Nil(err)
	result := map[string]string{}
	ttl, err := c.GetAndTTL(context.Background(), key, &result)
	assert.Nil(err)
	assert.NotEmpty(ttl)
	assert.Equal(data, result)
}

func TestCacheEmptyValue(t *testing.T) {
	assert := assert.New(t)

	// 无压缩：缓存空值应命中并返回空数据，而非ErrIsNil
	c, err := New(time.Minute)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck
	key := "empty"
	err = c.SetBytes(context.Background(), key, []byte{})
	assert.Nil(err)
	buf, err := c.GetBytes(context.Background(), key)
	assert.Nil(err)
	assert.Empty(buf)

	// 未命中仍返回 ErrNotFound / ErrIsNil
	_, err = c.GetBytes(context.Background(), "missing")
	assert.True(errors.Is(err, ErrNotFound))
	assert.True(errors.Is(err, ErrIsNil))

	// 有压缩：空值同样命中并正确解码
	cc, err := New(time.Minute, CacheSnappyOption(1))
	assert.Nil(err)
	defer cc.Close(context.Background()) //nolint:errcheck
	err = cc.SetBytes(context.Background(), key, []byte{})
	assert.Nil(err)
	buf, err = cc.GetBytes(context.Background(), key)
	assert.Nil(err)
	assert.Empty(buf)
}

// holdingStore keeps the last Set value by reference (no copy) to verify
// multi-store writers clone buffers before mutating timestamps.
type holdingStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newHoldingStore() *holdingStore {
	return &holdingStore{data: make(map[string][]byte)}
}

func (s *holdingStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value // intentionally no copy
	return nil
}

func (s *holdingStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (s *holdingStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *holdingStore) Close(context.Context) error { return nil }

func (s *holdingStore) snapshot(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.data[key]
	if v == nil {
		return nil
	}
	return append([]byte(nil), v...)
}

func TestMultiStoreClonesOnWrite(t *testing.T) {
	assert := assert.New(t)
	s1 := newHoldingStore()
	s2 := newHoldingStore()
	c, err := New(
		time.Minute,
		CacheStoreOption(s1),
		CacheSecondaryStoreOption(s2),
		CacheMultiTTLOption([]time.Duration{time.Second, 10 * time.Minute}),
	)
	assert.Nil(err)

	key := "k"
	value := []byte("payload")
	err = c.SetBytes(context.Background(), key, value)
	assert.Nil(err)

	b1 := s1.snapshot(key)
	b2 := s2.snapshot(key)
	assert.NotNil(b1)
	assert.NotNil(b2)
	// Payloads (after timestamp) must match
	assert.Equal(value, b1[timestampByteSize:])
	assert.Equal(value, b2[timestampByteSize:])
	// Embedded expiries must differ under multi-TTL (different hard timestamps)
	assert.NotEqual(b1[:timestampByteSize], b2[:timestampByteSize])
}

func TestPrimaryLogicalExpiryIsDeleted(t *testing.T) {
	assert := assert.New(t)
	s1 := newHoldingStore()
	s2 := newHoldingStore()

	// Seed secondary with a still-valid entry; primary with an already-expired one.
	key := "exp"
	payload := []byte("v")
	expired := make([]byte, timestampByteSize+len(payload))
	writeTimeToBytes(time.Now().Add(-time.Minute), expired)
	copy(expired[timestampByteSize:], payload)
	valid := make([]byte, timestampByteSize+len(payload))
	writeTimeToBytes(time.Now().Add(time.Minute), valid)
	copy(valid[timestampByteSize:], payload)
	_ = s1.Set(context.Background(), key, expired, time.Minute)
	_ = s2.Set(context.Background(), key, valid, time.Minute)

	c, err := New(
		time.Minute,
		CacheStoreOption(s1),
		CacheSecondaryStoreOption(s2),
	)
	assert.Nil(err)

	buf, err := c.GetBytes(context.Background(), key)
	assert.Nil(err)
	assert.Equal(payload, buf)
	// Primary expired entry should have been deleted then back-filled
	_, err = s1.Get(context.Background(), key)
	assert.Nil(err)
}

func TestSingleflightBackfill(t *testing.T) {
	assert := assert.New(t)

	var gets atomic.Int64
	secondary := &countingStore{
		Store: newHoldingStore(),
		gets:  &gets,
	}
	primary := newHoldingStore()

	key := "hot"
	payload := []byte("value")
	entry := make([]byte, timestampByteSize+len(payload))
	writeTimeToBytes(time.Now().Add(time.Minute), entry)
	copy(entry[timestampByteSize:], payload)
	_ = secondary.Set(context.Background(), key, entry, time.Minute)

	c, err := New(
		time.Minute,
		CacheStoreOption(primary),
		CacheSecondaryStoreOption(secondary),
	)
	assert.Nil(err)

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			buf, err := c.GetBytes(context.Background(), key)
			assert.Nil(err)
			assert.Equal(payload, buf)
		}()
	}
	wg.Wait()

	// Secondary Get should be coalesced by singleflight (typically 1; allow a
	// small bound in case of timing races across the outer fast-path miss).
	assert.LessOrEqual(gets.Load(), int64(4))
	assert.GreaterOrEqual(gets.Load(), int64(1))
}

type countingStore struct {
	Store
	gets *atomic.Int64
}

func (s *countingStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.gets.Add(1)
	return s.Store.Get(ctx, key)
}

func TestNewUsesPrimaryTTLForBigcache(t *testing.T) {
	assert := assert.New(t)
	// Multi-TTL primary is short; bigcache hard life should follow primary TTL
	// (ttlList[0]), not the New(ttl) argument.
	c, err := New(
		10*time.Minute,
		CacheMultiTTLOption([]time.Duration{
			50 * time.Millisecond,
			10 * time.Minute,
		}),
	)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := "k"
	err = c.SetBytes(context.Background(), key, []byte("v"))
	assert.Nil(err)

	// After primary soft TTL, getBytes treats entry as expired and deletes it.
	// Without a secondary store the read is a miss.
	time.Sleep(80 * time.Millisecond)
	_, err = c.GetBytes(context.Background(), key)
	assert.True(errors.Is(err, ErrNotFound))
}

// 删除不存在的key应视为成功，不同store对缺失key的返回不一致
// (bigcache返回错误，redis不返回)，Cache需统一屏蔽
func TestCacheDeleteNotExists(t *testing.T) {
	assert := assert.New(t)

	s1, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)
	s2, err := newBigCacheStore(time.Minute, &Option{})
	assert.Nil(err)

	c, err := New(
		time.Minute,
		CacheStoreOption(s1),
		CacheSecondaryStoreOption(s2),
	)
	assert.Nil(err)
	defer c.Close(context.Background()) //nolint:errcheck

	key := randomString()
	assert.Nil(c.Delete(context.Background(), key))

	// 只有一级缓存存在时，删除同样不应出错
	assert.Nil(s1.Set(context.Background(), key, make([]byte, timestampByteSize), time.Minute))
	assert.Nil(c.Delete(context.Background(), key))
	_, err = s1.Get(context.Background(), key)
	assert.True(errors.Is(err, ErrNotFound))

	// store本身仍需暴露ErrNotFound
	assert.True(errors.Is(s1.Delete(context.Background(), key), ErrNotFound))
}

func BenchmarkBigcache(b *testing.B) {
	c, _ := New(time.Minute, CacheHardMaxCacheSizeOption(1))
	for range b.N {
		key := randomString()
		err := c.SetBytes(context.Background(), key, []byte(key), time.Second)
		if err != nil {
			panic(err)
		}
	}
}

func benchmarkSetBytes(b *testing.B, opts ...CacheOption) {
	c, err := New(time.Minute, opts...)
	if err != nil {
		panic(err)
	}
	defer c.Close(context.Background()) //nolint:errcheck
	value := []byte(strings.Repeat("go-cache benchmark payload ", 16))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := c.SetBytes(context.Background(), "key", value); err != nil {
			panic(err)
		}
	}
}

func BenchmarkSetBytes(b *testing.B) {
	benchmarkSetBytes(b)
}

func BenchmarkSetBytesSnappy(b *testing.B) {
	benchmarkSetBytes(b, CacheSnappyOption(50))
}

func BenchmarkSetBytesZSTD(b *testing.B) {
	benchmarkSetBytes(b, CacheZSTDOption(50, 1))
}
