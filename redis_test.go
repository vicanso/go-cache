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
	"math/rand"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// randomString 生成随机的字符串
func randomString() string {
	n := 10
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func newClient() *redis.Client {
	c := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return c
}

func TestRedisLock(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	srv.SetPrefix("prefix:")
	// 设置默认的ttl
	srv.SetTTL(5 * time.Millisecond)
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

	// 首次成功
	ok, done, err := srv.LockWithDone(context.TODO(), key, 5*time.Millisecond)
	assert.Nil(err)
	assert.True(ok)

	// 第二次失败
	ok, _, err = srv.LockWithDone(context.TODO(), key, 5*time.Millisecond)
	assert.Nil(err)
	assert.False(ok)

	// 删除数据后第三次成功
	err = done()
	assert.Nil(err)
	ok, _, err = srv.LockWithDone(context.TODO(), key, 5*time.Millisecond)
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
}

func TestRedisGetSetStructSnappy(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	srv := NewRedisCache(c)
	key := randomString()
	type T struct {
		Name string `json:"name,omitempty"`
	}
	name := "Snappy 是一个 C++ 的用来压缩和解压缩的开发包。其目标不是最大限度压缩或者兼容其他压缩格式，而是旨在提供高速压缩速度和合理的压缩率。Snappy 比 zlib 更快，但文件相对要大 20% 到 100%。在 64位模式的 Core i7 处理器上，可达每秒 250~500兆的压缩速度。"
	err := srv.SetStructSnappy(context.TODO(), key, &T{
		Name: name,
	}, time.Minute)
	assert.Nil(err)

	result := T{}
	err = srv.GetStructSnappy(context.TODO(), key, &result)
	assert.Nil(err)
	assert.Equal(name, result.Name)
}
