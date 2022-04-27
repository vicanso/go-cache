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

func TestCompressor(t *testing.T) {
	assert := assert.New(t)

	shortString := `{"name":"test"}`
	longString := `{"name":"Snappy Snappy Snappy Snappy Snappy 速度很快"}`
	tests := []struct {
		Compressor   Compressor
		Data         []byte
		CompressData []byte
	}{
		{
			Compressor:   NewSnappyCompressor(50),
			Data:         []byte(shortString),
			CompressData: []byte("\x00{\"name\":\"test\"}"),
		},
		{
			Compressor:   NewSnappyCompressor(50),
			Data:         []byte(longString),
			CompressData: []byte("\x01:<{\"name\":\"Snappy n\a\x004速度很快\"}"),
		},
		{
			Compressor:   NewZSTDCompressor(50, 1),
			Data:         []byte(shortString),
			CompressData: []byte("\x00{\"name\":\"test\"}"),
		},
		{
			Compressor:   NewZSTDCompressor(50, 1),
			Data:         []byte(longString),
			CompressData: []byte("\x01(\xb5/\xfd\x04\x005\x01\x00\xe4\x01{\"name\":\"Snappy 速度很快\"}\x01T\x10\x03\x19\x14\x056\xcfS"),
		},
	}

	for _, tt := range tests {

		buf, err := tt.Compressor.Encode(tt.Data)
		assert.Nil(err)
		assert.Equal(tt.CompressData, buf)
		result, err := tt.Compressor.Decode(buf)
		assert.Nil(err)
		assert.Equal(tt.Data, result)
	}
}
