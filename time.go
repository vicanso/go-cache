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
	"encoding/binary"
	"time"
)

const timestampByteSize = 8

func writeTimeToBytes(t time.Time, data []byte) {
	timestamp := t.UnixNano()
	binary.BigEndian.PutUint64(data, uint64(timestamp))
}

func getTimeFromBytes(buf []byte) time.Time {
	timestamp := int64(binary.BigEndian.Uint64(buf))
	sec := timestamp / int64(time.Second)
	nsec := timestamp % int64(time.Second)

	return time.Unix(sec, nsec)
}
