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

type Marshaler interface {
	Marshal(v any) ([]byte, error)
}

func marshal(value any) ([]byte, error) {
	switch data := value.(type) {
	case *bytes.Buffer:
		return data.Bytes(), nil
	default:
		marshaler, ok := value.(Marshaler)
		fn := json.Marshal
		// 如果本身支持marshal
		if ok {
			fn = marshaler.Marshal
		}
		return fn(value)
	}
}
