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

// Compressor is the interface that support
// encode and decode function for compression.
//
// Match returns the size is matched for compress
type Compressor interface {
	// Match tests the data should be compressed
	Match(size int) (matched bool)
	// Encode encodes the data
	Encode(data []byte) ([]byte, error)
	// Decode decodes the data
	Decode(data []byte) ([]byte, error)
}
type CompressorOption struct {
	// MinCompressLength min compress length
	MinCompressLength int
	// Encode encode function
	Encode func(data []byte) ([]byte, error)
	// Decode decode function
	Decode func(data []byte) ([]byte, error)
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

type compressor struct {
	minCompressLength int
	encode            func(data []byte) ([]byte, error)
	decode            func(data []byte) ([]byte, error)
}

func (c *compressor) Encode(data []byte) ([]byte, error) {
	return c.encodeWithPrefix(0, data)
}

// prefixEncoder 是可选的内部优化接口。实现者将编码结果写入一个
// 头部预留了prefixLen字节空位的buffer，使调用方(Cache)可将时间戳
// 前缀与压缩头合并为一次内存分配，省去一次整段拷贝。
type prefixEncoder interface {
	encodeWithPrefix(prefixLen int, data []byte) ([]byte, error)
}

// encodeWithPrefix 在结果头部预留prefixLen字节空位，其后为1字节
// 压缩标志与(可选压缩的)数据。
func (c *compressor) encodeWithPrefix(prefixLen int, data []byte) ([]byte, error) {
	compressType := CompressNone
	if c.Match(len(data)) {
		buf, err := c.encode(data)
		if err != nil {
			return nil, err
		}
		data = buf
		compressType = Compressed
	}
	out := make([]byte, prefixLen+1+len(data))
	out[prefixLen] = compressType
	copy(out[prefixLen+1:], data)
	return out, nil
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

// NewCompressor creates a new compressor
func NewCompressor(opt CompressorOption) Compressor {
	return &compressor{
		minCompressLength: opt.MinCompressLength,
		encode:            opt.Encode,
		decode:            opt.Decode,
	}
}

// NewZSTDCompressor creates a zstd compressor. The underlying encoder and
// decoder are created once and reused across calls (EncodeAll and DecodeAll
// are safe for concurrent use), which avoids the goroutine leak and repeated
// allocation of creating a new writer/reader on every Encode/Decode.
func NewZSTDCompressor(minCompressLength, level int) Compressor {
	encoder, encErr := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	decoder, decErr := zstd.NewReader(nil)
	return NewCompressor(CompressorOption{
		MinCompressLength: minCompressLength,
		Encode: func(data []byte) ([]byte, error) {
			if encErr != nil {
				return nil, encErr
			}
			return encoder.EncodeAll(data, make([]byte, 0, len(data))), nil
		},
		Decode: func(data []byte) ([]byte, error) {
			if decErr != nil {
				return nil, decErr
			}
			return decoder.DecodeAll(data, nil)
		},
	})
}

// NewSnappyCompressor creates a snappy compressor
func NewSnappyCompressor(minCompressLength int) Compressor {
	return NewCompressor(CompressorOption{
		MinCompressLength: minCompressLength,
		Encode:            snappyEncode,
		Decode:            snappyDecode,
	})
}
