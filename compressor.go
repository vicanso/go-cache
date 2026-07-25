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

// errCompressorFuncNil is returned when a compressor is used without the
// required encode/decode function.
var errCompressorFuncNil = errors.New("compressor encode/decode function is nil")

// Compressor compresses values before they are handed to the stores.
//
// EncodeTo and DecodeTo have append semantics: they must append their result
// to dst and return the extended buffer, exactly like the builtin append.
// Cache relies on this to build a whole entry ([expiry][compress flag][body])
// in a single allocation, so an implementation that ignores dst and returns a
// fresh slice will corrupt the entry header.
//
// Implementations must be safe for concurrent use.
type Compressor interface {
	// Match tests whether a value of this size should be compressed
	Match(size int) (matched bool)
	// MaxEncodedLen returns an upper bound of the encoded size of n bytes.
	// It is only used to pre-size the destination buffer, so a value that is
	// too small stays correct (the buffer grows on demand), just less efficient.
	MaxEncodedLen(n int) int
	// EncodeTo appends the encoded form of data to dst
	EncodeTo(dst, data []byte) ([]byte, error)
	// DecodeTo appends the decoded form of data to dst
	DecodeTo(dst, data []byte) ([]byte, error)
}

// Closer is optionally implemented by compressors that hold reusable resources
// (e.g. zstd encoder/decoder). Cache.Close will call Close when available.
type Closer interface {
	Close() error
}

// CompressorOption builds a Compressor from plain encode/decode functions.
// The functions are adapted to the append based Compressor interface, which
// costs one extra copy per write; implement Compressor directly to avoid it.
type CompressorOption struct {
	// MinCompressLength min compress length
	MinCompressLength int
	// Encode encode function
	Encode func(data []byte) ([]byte, error)
	// Decode decode function
	Decode func(data []byte) ([]byte, error)
}

// growTail returns a buffer whose contents equal dst and whose spare capacity
// is at least n bytes, so a non-appending codec can write into dst[len:cap].
func growTail(dst []byte, n int) []byte {
	if cap(dst)-len(dst) >= n {
		return dst
	}
	grown := make([]byte, len(dst), len(dst)+n)
	copy(grown, dst)
	return grown
}

type compressor struct {
	minCompressLength int
	maxEncodedLen     func(n int) int
	encodeTo          func(dst, data []byte) ([]byte, error)
	decodeTo          func(dst, data []byte) ([]byte, error)
	close             func() error
}

func (c *compressor) Match(size int) bool {
	return size > c.minCompressLength
}

func (c *compressor) MaxEncodedLen(n int) int {
	if c.maxEncodedLen == nil {
		return n
	}
	return c.maxEncodedLen(n)
}

func (c *compressor) EncodeTo(dst, data []byte) ([]byte, error) {
	if c.encodeTo == nil {
		return nil, errCompressorFuncNil
	}
	return c.encodeTo(dst, data)
}

func (c *compressor) DecodeTo(dst, data []byte) ([]byte, error) {
	if c.decodeTo == nil {
		return nil, errCompressorFuncNil
	}
	return c.decodeTo(dst, data)
}

func (c *compressor) Close() error {
	if c.close != nil {
		return c.close()
	}
	return nil
}

// NewCompressor creates a new compressor
func NewCompressor(opt CompressorOption) Compressor {
	c := &compressor{
		minCompressLength: opt.MinCompressLength,
	}
	if opt.Encode != nil {
		c.encodeTo = func(dst, data []byte) ([]byte, error) {
			buf, err := opt.Encode(data)
			if err != nil {
				return nil, err
			}
			return append(dst, buf...), nil
		}
	}
	if opt.Decode != nil {
		c.decodeTo = func(dst, data []byte) ([]byte, error) {
			buf, err := opt.Decode(data)
			if err != nil {
				return nil, err
			}
			return append(dst, buf...), nil
		}
	}
	return c
}

// NewZSTDCompressor creates a zstd compressor. The underlying encoder and
// decoder are created once and reused across calls (EncodeAll and DecodeAll
// are safe for concurrent use), which avoids the goroutine leak and repeated
// allocation of creating a new writer/reader on every call.
// Call Close (via Cache.Close when used as the cache compressor) to release them.
func NewZSTDCompressor(minCompressLength, level int) Compressor {
	encoder, encErr := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	decoder, decErr := zstd.NewReader(nil)
	return &compressor{
		minCompressLength: minCompressLength,
		maxEncodedLen: func(n int) int {
			if encErr != nil {
				return n
			}
			return encoder.MaxEncodedSize(n)
		},
		// EncodeAll/DecodeAll 本身即为append语义，可直接写入调用方的buffer
		encodeTo: func(dst, data []byte) ([]byte, error) {
			if encErr != nil {
				return nil, encErr
			}
			return encoder.EncodeAll(data, dst), nil
		},
		decodeTo: func(dst, data []byte) ([]byte, error) {
			if decErr != nil {
				return nil, decErr
			}
			return decoder.DecodeAll(data, dst)
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

// NewSnappyCompressor creates a snappy compressor
func NewSnappyCompressor(minCompressLength int) Compressor {
	return &compressor{
		minCompressLength: minCompressLength,
		maxEncodedLen:     snappy.MaxEncodedLen,
		// snappy无append语义，改为写入dst尾部的空闲容量后再重新切片
		encodeTo: func(dst, data []byte) ([]byte, error) {
			dst = growTail(dst, snappy.MaxEncodedLen(len(data)))
			n := len(dst)
			encoded := snappy.Encode(dst[n:cap(dst)], data)
			return dst[:n+len(encoded)], nil
		},
		decodeTo: func(dst, data []byte) ([]byte, error) {
			size, err := snappy.DecodedLen(data)
			if err != nil {
				return nil, err
			}
			dst = growTail(dst, size)
			n := len(dst)
			decoded, err := snappy.Decode(dst[n:n+size], data)
			if err != nil {
				return nil, err
			}
			return dst[:n+len(decoded)], nil
		},
	}
}
