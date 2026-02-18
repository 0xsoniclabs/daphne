// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package tracker

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Metadata is a key/value map used for storing Metadata in tracker entries.
type Metadata struct {
	data map[string]any
}

// Keys returns a sorted slice of all keys in the metadata.
func (m Metadata) Keys() []string {
	keys := slices.Collect(maps.Keys(m.data))
	slices.Sort(keys)
	return keys
}

// Get retrieves the value associated with the given key. If no value is bound
// to the key, it returns an empty string.
func (m Metadata) Get(key string) any {
	return m.data[key]
}

// String returns a string representation of the metadata, with keys sorted
// in alphabetical order. The format is "{key1: value1, key2: value2, ...}".
func (m Metadata) String() string {
	keys := slices.Collect(maps.Keys(m.data))
	slices.Sort(keys)
	elements := make([]string, 0, len(keys))
	for _, key := range keys {
		value := m.data[key]
		elements = append(elements, fmt.Sprintf("%s: %v", key, value))
	}
	return fmt.Sprintf("{%s}", strings.Join(elements, ", "))
}

// toMeta converts a variable number of key/value pairs into metadata. Each
// key and value is converted to a string using fmt.Sprintf. The keys and values
// are expected to be provided in pairs, so the function processes them in steps
// of two. If an odd number of arguments is provided, the last one is ignored.
func toMeta(meta ...any) Metadata {
	result := make(map[string]any)
	for i := 0; i < len(meta); i += 2 {
		if i+1 < len(meta) {
			key := fmt.Sprintf("%v", meta[i])
			value := meta[i+1]
			result[key] = value
		}
	}
	return Metadata{data: result}
}

// merge merges another meta map into the current one. It updates the current
// map with the key/value pairs from the other map, overwriting any existing
// keys with the values from the other map. It returns a pointer to the updated
// metadata map.
func (m *Metadata) merge(other Metadata) *Metadata {
	maps.Copy(m.data, other.data)
	return m
}

// flatten converts the metadata map into a slice of alternating keys and values.
func (m *Metadata) flatten() []any {
	result := make([]any, 0, len(m.data)*2)
	for k, v := range m.data {
		result = append(result, k, v)
	}
	return result
}
