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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	lruttl "github.com/vicanso/lru-ttl"
)

func TestMultiCache(t *testing.T) {
	type TestData struct {
		Name string `json:"name,omitempty"`
	}

	assert := assert.New(t)
	c := newClient()
	srv := NewRedisCache(c)

	mc := NewMultilevelCache(MultilevelCacheOptions{
		Cache:   srv,
		TTL:     time.Minute,
		LRUSize: 1,
		Prefix:  "multilevel:",
	})

	data := TestData{
		Name: "nickname",
	}

	key := randomString()
	// 首次无数据
	err := mc.Get(key, &TestData{})
	assert.Equal(lruttl.ErrIsNil, err)

	// 设置数据后，查询成功（从lru获取)
	err = mc.Set(key, &data)
	assert.Nil(err)
	result := TestData{}
	err = mc.Get(key, &result)
	assert.Nil(err)
	assert.Equal(data.Name, result.Name)

	// 添加新的数据，lru的数据被更新
	err = mc.Set("a", &TestData{})
	assert.Nil(err)
	result = TestData{}
	// 从redis中获取数据
	err = mc.Get(key, &result)
	assert.Nil(err)
	assert.Equal(data.Name, result.Name)
}
