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
	"encoding/json"

	"github.com/golang/snappy"
)

type compressor struct {
	opts *CompressorOptions
	// // minCompressLength is the min length to compress
	// minCompressLength int
	// // encode compress encode function
	// encode func([]byte) ([]byte, error)
	// // decode compress decode function
	// decode func([]byte) ([]byte, error)
}

type CompressorOptions struct {
	// MinCompressLength is the min length to compress
	MinCompressLength int
	// Encode compress encode function
	Encode func([]byte) ([]byte, error)
	// Decode compress decode function
	Decode func([]byte) ([]byte, error)
}

func snappyEncode(data []byte) ([]byte, error) {
	dst := []byte{}
	dst = snappy.Encode(dst, data)
	return dst, nil
}

func snappyDecode(buf []byte) ([]byte, error) {
	var dst []byte
	return snappy.Decode(dst, buf)
}

// Marshal returns the data marshal by json and compress by decoder.
// If the size of data <= minCompressLength, it will not compressed.
func (c *compressor) Marshal(v interface{}) ([]byte, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// 不做压缩
	size := len(buf)
	compressType := CompressNone
	opts := c.opts
	if size > opts.MinCompressLength {
		compressType = Compressed
		buf, err = opts.Encode(buf)
		if err != nil {
			return nil, err
		}
		size = len(buf)
	}
	newData := make([]byte, size+1)
	newData[0] = compressType
	copy(newData[1:], buf)
	return newData, nil
}

// Unmarshal decode data by decoder and use json unmarshal to result
func (c *compressor) Unmarshal(data []byte, result interface{}) error {
	if len(data) == 0 {
		return nil
	}
	compressType := data[0]
	buf := data[1:]
	if compressType != CompressNone {
		data, err := c.opts.Decode(buf)
		if err != nil {
			return err
		}
		buf = data
	}
	return json.Unmarshal(buf, result)
}

// NewSnappyCompressor returns a new snappy compressor
func NewSnappyCompressor(minCompressLength int) *compressor {
	return NewComprsser(CompressorOptions{
		MinCompressLength: minCompressLength,
		Encode:            snappyEncode,
		Decode:            snappyDecode,
	})
}

// NewComprsser returns a new compressor
func NewComprsser(opts CompressorOptions) *compressor {
	return &compressor{
		opts: &opts,
	}
}
