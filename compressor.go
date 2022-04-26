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
	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

type Compressor interface {
	Match(size int) (matched bool)
	Encode(data []byte) ([]byte, error)
	Decode(data []byte) ([]byte, error)
}
type CompressorOption struct {
	MinCompressLength int
	Encode            func(data []byte) ([]byte, error)
	Decode            func(data []byte) ([]byte, error)
}

func snappyEncode(data []byte) ([]byte, error) {
	dst := []byte{}
	dst = snappy.Encode(dst, data)
	return dst, nil
}

func snappyDecode(data []byte) ([]byte, error) {
	var dst []byte
	return snappy.Decode(dst, data)
}

func zstdEncode(data []byte, level int) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	if err != nil {
		return nil, err
	}
	data = encoder.EncodeAll(data, make([]byte, 0, len(data)))
	return data, nil
}

func zstdDecode(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	return decoder.DecodeAll(data, nil)
}

type compressor struct {
	minCompressLength int
	encode            func(data []byte) ([]byte, error)
	decode            func(data []byte) ([]byte, error)
}

func (c *compressor) Encode(data []byte) ([]byte, error) {
	size := len(data)
	// 不做压缩
	compressType := CompressNone
	if c.Match(size) {
		compressType = Compressed
		buf, err := c.encode(data)
		if err != nil {
			return nil, err
		}
		data = buf
		size = len(data)
	}
	newData := make([]byte, size+1)
	newData[0] = compressType
	copy(newData[1:], data)
	return newData, nil
}

func (c *compressor) Decode(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	compressType := data[0]
	data = data[1:]
	if compressType != CompressNone {
		buf, err := c.decode(data)
		if err != nil {
			return nil, err
		}
		data = buf
	}
	return data, nil
}

func (c *compressor) Match(size int) bool {
	return size > c.minCompressLength
}

func NewCompressor(opt CompressorOption) Compressor {
	return &compressor{
		minCompressLength: opt.MinCompressLength,
		encode:            opt.Encode,
		decode:            opt.Decode,
	}
}

func NewZSTDCompressor(minCompressLength, level int) Compressor {
	return NewCompressor(CompressorOption{
		MinCompressLength: minCompressLength,
		Encode: func(data []byte) ([]byte, error) {
			return zstdEncode(data, level)
		},
		Decode: zstdDecode,
	})
}

func NewSnappyCompressor(minCompressLength int) Compressor {
	return NewCompressor(CompressorOption{
		MinCompressLength: minCompressLength,
		Encode:            snappyEncode,
		Decode:            snappyDecode,
	})
}
