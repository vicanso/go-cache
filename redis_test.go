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
	"crypto/rand"

	redis "github.com/redis/go-redis/v9"
)

// randomString 生成随机的字符串
func randomString() string {
	n := 10
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return string(b)
}

func newClient() *redis.Client {
	c := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return c
}
