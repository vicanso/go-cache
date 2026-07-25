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
	"bytes"
	"encoding/json"
)

type Unmarshaler interface {
	Unmarshal(data []byte, value any) error
}

func unmarshal(data []byte, value any) error {
	switch v := value.(type) {
	case *bytes.Buffer:
		// 先清空，否则复用同一个buffer连续读取会把多次结果拼接在一起
		v.Reset()
		_, err := v.Write(data)
		return err
	default:
		fn := json.Unmarshal
		unmarshaler, ok := value.(Unmarshaler)
		if ok {
			fn = unmarshaler.Unmarshal
		}
		return fn(data, value)
	}
}
