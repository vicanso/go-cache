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

// encodeWithPrefix 预留的前缀不能影响编码结果，且长度必须精确
// (snappy走的是预分配容量+截断的路径，长度算错会带上垃圾数据)
func TestCompressorEncodeWithPrefix(t *testing.T) {
	assert := assert.New(t)

	shortString := []byte(`{"name":"test"}`)
	longString := []byte(`{"name":"Snappy Snappy Snappy Snappy Snappy 速度很快"}`)
	compressors := []Compressor{
		NewSnappyCompressor(50),
		NewZSTDCompressor(50, 1),
		NewCompressor(CompressorOption{
			MinCompressLength: 50,
			Encode:            func(data []byte) ([]byte, error) { return data, nil },
			Decode:            func(data []byte) ([]byte, error) { return data, nil },
		}),
	}
	const prefixLen = timestampByteSize

	for _, c := range compressors {
		pe, ok := c.(prefixEncoder)
		assert.True(ok)
		for _, data := range [][]byte{shortString, longString} {
			noPrefix, err := c.Encode(data)
			assert.Nil(err)

			buf, err := pe.encodeWithPrefix(prefixLen, data)
			assert.Nil(err)
			// 前缀之后的内容应与不带前缀的编码结果完全一致
			assert.Equal(noPrefix, buf[prefixLen:])

			result, err := c.Decode(buf[prefixLen:])
			assert.Nil(err)
			assert.Equal(data, result)
		}
	}
}

// 自定义压缩器未提供Encode/Decode时应返回错误而非panic
func TestCompressorNilFunc(t *testing.T) {
	assert := assert.New(t)

	c := NewCompressor(CompressorOption{
		MinCompressLength: 1,
	})
	_, err := c.Encode([]byte("abcd"))
	assert.Equal(errCompressorFuncNil, err)

	_, err = c.Decode([]byte{Compressed, 'a'})
	assert.Equal(errCompressorFuncNil, err)

	// 未触发压缩时不需要Encode/Decode
	buf, err := c.Encode(nil)
	assert.Nil(err)
	assert.Equal([]byte{CompressNone}, buf)
	result, err := c.Decode(buf)
	assert.Nil(err)
	assert.Empty(result)
}
