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

	c, err := New(1 * time.Second)
	assert.Nil(err)
	defer c.Close(context.Background())

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

	time.Sleep(2 * time.Second)
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
	defer c.Close(context.Background())

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
	defer c.Close(context.Background())

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

	_, err = s1.Get(context.Background(), key)
	assert.Equal(ErrIsNil, err)
	_, err = s2.Get(context.Background(), key)
	assert.Equal(ErrIsNil, err)
}

func BenchmarkBigcache(b *testing.B) {
	c, _ := New(time.Minute, CacheHardMaxCacheSizeOption(1))
	for i := 0; i < b.N; i++ {
		key := randomString()
		err := c.SetBytes(context.Background(), key, []byte(key), time.Second)
		if err != nil {
			panic(err)
		}
	}
}
