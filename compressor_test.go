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

	longString := []byte(`{"name":"Snappy Snappy Snappy Snappy Snappy 速度很快"}`)
	tests := []struct {
		Compressor   Compressor
		Data         []byte
		CompressData []byte
	}{
		{
			Compressor:   NewSnappyCompressor(50),
			Data:         longString,
			CompressData: []byte(":<{\"name\":\"Snappy n\a\x004速度很快\"}"),
		},
		{
			Compressor:   NewZSTDCompressor(50, 1),
			Data:         longString,
			CompressData: []byte("(\xb5/\xfd\x04\x005\x01\x00\xe4\x01{\"name\":\"Snappy 速度很快\"}\x01T\x10\x03\x19\x14\x056\xcfS"),
		},
	}

	for _, tt := range tests {
		assert.True(tt.Compressor.Match(len(tt.Data)))
		assert.GreaterOrEqual(tt.Compressor.MaxEncodedLen(len(tt.Data)), len(tt.CompressData))

		buf, err := tt.Compressor.EncodeTo(nil, tt.Data)
		assert.Nil(err)
		assert.Equal(tt.CompressData, buf)

		result, err := tt.Compressor.DecodeTo(nil, buf)
		assert.Nil(err)
		assert.Equal(tt.Data, result)
	}
}

// EncodeTo/DecodeTo 必须是append语义：已有内容不能被覆盖，
// 返回长度必须精确(snappy走的是预分配容量再截断的路径)
func TestCompressorAppendSemantics(t *testing.T) {
	assert := assert.New(t)

	data := []byte(`{"name":"Snappy Snappy Snappy Snappy Snappy 速度很快"}`)
	compressors := []Compressor{
		NewSnappyCompressor(50),
		NewZSTDCompressor(50, 1),
		NewCompressor(CompressorOption{
			MinCompressLength: 50,
			Encode:            func(data []byte) ([]byte, error) { return data, nil },
			Decode:            func(data []byte) ([]byte, error) { return data, nil },
		}),
	}
	prefix := []byte("prefix")

	for _, c := range compressors {
		plain, err := c.EncodeTo(nil, data)
		assert.Nil(err)

		// 容量充足(调用方已按MaxEncodedLen预留)与容量不足两种情形都要正确
		for _, dst := range [][]byte{
			append(make([]byte, 0, len(prefix)+c.MaxEncodedLen(len(data))), prefix...),
			append([]byte(nil), prefix...),
		} {
			buf, err := c.EncodeTo(dst, data)
			assert.Nil(err)
			assert.Equal(prefix, buf[:len(prefix)])
			assert.Equal(plain, buf[len(prefix):])

			result, err := c.DecodeTo(append([]byte(nil), prefix...), buf[len(prefix):])
			assert.Nil(err)
			assert.Equal(prefix, result[:len(prefix)])
			assert.Equal(data, result[len(prefix):])
		}
	}
}

// 自定义压缩器未提供Encode/Decode时应返回错误而非panic
func TestCompressorNilFunc(t *testing.T) {
	assert := assert.New(t)

	c := NewCompressor(CompressorOption{
		MinCompressLength: 1,
	})
	_, err := c.EncodeTo(nil, []byte("abcd"))
	assert.Equal(errCompressorFuncNil, err)

	_, err = c.DecodeTo(nil, []byte("abcd"))
	assert.Equal(errCompressorFuncNil, err)

	// 未提供maxEncodedLen时回退为原始长度
	assert.Equal(4, c.MaxEncodedLen(4))
}
