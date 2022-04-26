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

func TestGetSet(t *testing.T) {
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

	err = c.Get(context.Background(), "abc", nil)
	assert.Equal(ErrNotFound, err)

	time.Sleep(2 * time.Second)
	err = c.Get(context.Background(), keyCustom, nil)
	assert.Equal(ErrNotFound, err)
}
