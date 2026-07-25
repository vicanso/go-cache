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
	"errors"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

// errCompressorFuncNil is returned when a custom compressor is used without
// the required Encode/Decode function.
var errCompressorFuncNil = errors.New("compressor encode/decode function is nil")

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

// Closer is optionally implemented by compressors that hold reusable resources
// (e.g. zstd encoder/decoder). Cache.Close will call Close when available.
type Closer interface {
	Close() error
}

type CompressorOption struct {
	// MinCompressLength min compress length
	MinCompressLength int
	// Encode encode function
	Encode func(data []byte) ([]byte, error)
	// Decode decode function
	Decode func(data []byte) ([]byte, error)
}

func snappyDecode(data []byte) ([]byte, error) {
	n, err := snappy.DecodedLen(data)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, n)
	return snappy.Decode(dst, data)
}

type compressor struct {
	minCompressLength int
	encode            func(data []byte) ([]byte, error)
	decode            func(data []byte) ([]byte, error)
	// encodeAppend 将压缩结果追加到dst尾部并返回扩展后的slice。
	// 内置压缩器均实现该函数，使encodeWithPrefix可以一次分配出
	// [调用方前缀][压缩标志][压缩数据]，省去一次整段拷贝。
	encodeAppend func(dst, data []byte) ([]byte, error)
	// maxEncodedLen 返回压缩后长度上限，仅用于预分配容量，
	// 返回值偏小不影响正确性(append会自行扩容)。
	maxEncodedLen func(n int) int
	close         func() error
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
	if !c.Match(len(data)) {
		out := make([]byte, prefixLen+1+len(data))
		out[prefixLen] = CompressNone
		copy(out[prefixLen+1:], data)
		return out, nil
	}
	// 压缩器支持追加写入时，直接压缩到预留了前缀与标志位的buffer中
	if c.encodeAppend != nil {
		size := len(data)
		if c.maxEncodedLen != nil {
			size = c.maxEncodedLen(size)
		}
		out := make([]byte, prefixLen+1, prefixLen+1+size)
		out[prefixLen] = Compressed
		return c.encodeAppend(out, data)
	}
	if c.encode == nil {
		return nil, errCompressorFuncNil
	}
	buf, err := c.encode(data)
	if err != nil {
		return nil, err
	}
	out := make([]byte, prefixLen+1+len(buf))
	out[prefixLen] = Compressed
	copy(out[prefixLen+1:], buf)
	return out, nil
}

func (c *compressor) Decode(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	compressType := data[0]
	data = data[1:]
	if compressType != CompressNone {
		if c.decode == nil {
			return nil, errCompressorFuncNil
		}
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

func (c *compressor) Close() error {
	if c.close != nil {
		return c.close()
	}
	return nil
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
// Call Close (via Cache.Close when used as the cache compressor) to release them.
func NewZSTDCompressor(minCompressLength, level int) Compressor {
	encoder, encErr := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	decoder, decErr := zstd.NewReader(nil)
	return &compressor{
		minCompressLength: minCompressLength,
		// EncodeAll 本身即为append语义，可直接写入预留了前缀的buffer
		encodeAppend: func(dst, data []byte) ([]byte, error) {
			if encErr != nil {
				return nil, encErr
			}
			return encoder.EncodeAll(data, dst), nil
		},
		maxEncodedLen: func(n int) int {
			if encErr != nil {
				return n
			}
			return encoder.MaxEncodedSize(n)
		},
		decode: func(data []byte) ([]byte, error) {
			if decErr != nil {
				return nil, decErr
			}
			return decoder.DecodeAll(data, nil)
		},
		close: func() error {
			if encoder != nil {
				_ = encoder.Close()
			}
			if decoder != nil {
				decoder.Close()
			}
			return nil
		},
	}
}

// NewSnappyCompressor creates a snappy compressor with a single-allocation
// encodeWithPrefix path (timestamp prefix + compress flag + body).
func NewSnappyCompressor(minCompressLength int) Compressor {
	return &compressor{
		minCompressLength: minCompressLength,
		// snappy.Encode 无append语义，改为写入dst尾部的空闲容量，
		// 容量由maxEncodedLen保证充足
		encodeAppend: func(dst, data []byte) ([]byte, error) {
			n := len(dst)
			encoded := snappy.Encode(dst[n:cap(dst)], data)
			return dst[:n+len(encoded)], nil
		},
		maxEncodedLen: snappy.MaxEncodedLen,
		decode:        snappyDecode,
	}
}
