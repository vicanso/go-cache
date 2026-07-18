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
	"testing"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

// randomString 生成随机的字符串
func randomString() string {
	n := 10
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return string(b)
}

// newClient 启动一个内存版miniredis并返回连接它的client，
// 测试结束时自动关闭。返回的miniredis实例可用于FastForward等
// 需要手动推进时间的场景(miniredis不会随真实时间自动过期key)。
func newClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	}), mr
}
