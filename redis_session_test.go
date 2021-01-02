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
)

func TestRedisSession(t *testing.T) {
	assert := assert.New(t)
	c := newClient()
	defer c.Close()
	rs := NewRedisSession(c)
	key := randomString()
	rs.SetPrefix("ss:")

	data := []byte("abcd")
	err := rs.Set(key, data, time.Minute)
	assert.Nil(err)

	result, err := rs.Get(key)
	assert.Nil(err)
	assert.Equal(data, result)

	// 删除
	err = rs.Destroy(key)
	assert.Nil(err)

	// 删除后获取，为空
	result, err = rs.Get(key)
	assert.Nil(err)
	assert.Empty(result)
}
