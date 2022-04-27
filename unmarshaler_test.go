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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	assert := assert.New(t)

	b := bytes.NewBufferString("")
	err := unmarshal([]byte("abc"), b)
	assert.Nil(err)
	assert.Equal([]byte("abc"), b.Bytes())

	m := make(map[string]string)
	err = unmarshal([]byte(`{"name":"abc"}`), &m)
	assert.Nil(err)
	assert.Equal("abc", m["name"])
}
