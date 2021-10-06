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

	"github.com/stretchr/testify/assert"
)

func TestSnappyCompressor(t *testing.T) {
	assert := assert.New(t)
	type Data struct {
		Name string `json:"name,omitempty"`
	}

	sc := NewSnappyCompressor(50)
	data := &Data{
		Name: "test",
	}
	buf, err := sc.Marshal(data)
	assert.Nil(err)
	assert.Equal("\x00{\"name\":\"test\"}", string(buf))
	unmarshalData := &Data{}
	err = sc.Unmarshal(buf, unmarshalData)
	assert.Nil(err)
	assert.Equal(data.Name, unmarshalData.Name)

	data = &Data{
		Name: "Snappy Snappy Snappy Snappy Snappy 速度很快",
	}
	buf, err = sc.Marshal(data)
	assert.Nil(err)
	assert.Equal("\x01:<{\"name\":\"Snappy n\a\x004速度很快\"}", string(buf))
	unmarshalData = &Data{}
	err = sc.Unmarshal(buf, unmarshalData)
	assert.Nil(err)
	assert.Equal(data.Name, unmarshalData.Name)

}

func TestZSTDCompressor(t *testing.T) {
	assert := assert.New(t)
	type Data struct {
		Name string `json:"name,omitempty"`
	}

	sc := NewZSTDCompressor(50)
	data := &Data{
		Name: "test",
	}
	buf, err := sc.Marshal(data)
	assert.Nil(err)
	assert.Equal("\x00{\"name\":\"test\"}", string(buf))
	unmarshalData := &Data{}
	err = sc.Unmarshal(buf, unmarshalData)
	assert.Nil(err)
	assert.Equal(data.Name, unmarshalData.Name)

	data = &Data{
		Name: "Snappy Snappy Snappy Snappy Snappy 速度很快",
	}
	buf, err = sc.Marshal(data)
	assert.Nil(err)
	assert.Equal("\x01(\xb5/\xfd\x04\x005\x01\x00\xe4\x01{\"name\":\"Snappy 速度很快\"}\x01T\x10\x03\x19\x14\x056\xcfS", string(buf))
	unmarshalData = &Data{}
	err = sc.Unmarshal(buf, unmarshalData)
	assert.Nil(err)
	assert.Equal(data.Name, unmarshalData.Name)
}
