// Copyright 2021 tree xie
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

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisLock(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	opts := []RedisCacheOption{
		RedisCachePrefixOption("prefix:"),
		RedisCacheTTLOption(5 * time.Millisecond),
	}
	srv := NewRedisCache(c, opts...)
	key := randomString()

	// 首次成功
	ok, err := srv.Lock(context.TODO(), key)
	assert.Nil(err)
	assert.True(ok)

	// 第二次失败
	ok, err = srv.Lock(context.TODO(), key)
	assert.Nil(err)
	assert.False(ok)

	// 第三次等待过期后成功
	time.Sleep(10 * time.Millisecond)
	ok, err = srv.Lock(context.TODO(), key)
	assert.Nil(err)
	assert.True(ok)
}

func TestRedisLockWithDone(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	key := randomString()
	ttl := 2 * time.Second

	// 首次成功
	ok, done, err := srv.LockWithDone(context.TODO(), key, ttl)
	assert.Nil(err)
	assert.True(ok)

	// 第二次失败
	ok, _, err = srv.LockWithDone(context.TODO(), key, ttl)
	assert.Nil(err)
	assert.False(ok)

	// 删除数据后第三次成功
	err = done()
	assert.Nil(err)
	ok, _, err = srv.LockWithDone(context.TODO(), key, ttl)
	assert.Nil(err)
	assert.True(ok)
}

func TestRedisIncWithTTL(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	key := randomString()

	count, err := srv.IncWith(context.TODO(), key, 1, time.Minute)
	assert.Nil(err)
	assert.Equal(int64(1), count)

	count, err = srv.IncWith(context.TODO(), key, 2, time.Minute)
	assert.Nil(err)
	assert.Equal(int64(3), count)

	count, err = srv.IncWith(context.TODO(), key, -4, time.Minute)
	assert.Nil(err)
	assert.Equal(int64(-1), count)

	count, err = srv.Del(context.TODO(), key)
	assert.Nil(err)
	assert.Equal(int64(1), count)
}

func TestRedisGetSet(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	key := randomString()

	// 获取不存在时，返回出错
	_, err := srv.Get(context.TODO(), key)
	assert.Equal(redis.Nil, err)

	// 获取不存在时，忽略Nil Error
	result, err := srv.GetIgnoreNilErr(context.TODO(), key)
	assert.Nil(err)
	assert.Empty(result)

	// 设置成功
	value := "abc"
	err = srv.Set(context.TODO(), key, value, time.Minute)
	assert.Nil(err)

	// 获取成功
	result, err = srv.Get(context.TODO(), key)
	assert.Nil(err)
	assert.Equal(value, string(result))

	// 获取后删除
	result, err = srv.GetAndDel(context.TODO(), key)
	assert.Nil(err)
	assert.Equal(value, string(result))

	// 再次获取则不存在
	_, err = srv.Get(context.TODO(), key)
	assert.Equal(redis.Nil, err)
}

func TestRedisGetSetStruct(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	key := randomString()

	type T struct {
		Name string `json:"name,omitempty"`
	}
	name := "abc"
	err := srv.SetStruct(context.TODO(), key, &T{
		Name: name,
	}, time.Minute)
	assert.Nil(err)

	result := T{}
	err = srv.GetStruct(context.TODO(), key, &result)
	assert.Nil(err)
	assert.Equal(name, result.Name)

	result = T{}
	done, err := srv.GetStructWithDone(context.TODO(), key, &result)
	assert.Nil(err)
	assert.Equal(name, result.Name)
	err = done()
	assert.Nil(err)

	// 再次获取则不存在
	_, err = srv.Get(context.TODO(), key)
	assert.Equal(redis.Nil, err)
}

func TestRedisGetSetStructTTL(t *testing.T) {
	assert := assert.New(t)

	data := map[string]string{
		"a": "1",
	}

	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	err := srv.SetStructWithTTL(context.Background(), "key", data, 10*time.Second)
	assert.Nil(err)
	time.Sleep(50 * time.Millisecond)

	value := map[string]string{}
	ttl, err := srv.GetStructAndTTL(context.Background(), "key", &value)
	assert.Nil(err)
	assert.Equal(data, value)
	assert.Less(ttl, 10*time.Second)
	assert.Greater(ttl, 9*time.Second)

	ttl, err = srv.TTL(context.Background(), "key")
	assert.Nil(err)
	assert.NotEmpty(ttl)
}
