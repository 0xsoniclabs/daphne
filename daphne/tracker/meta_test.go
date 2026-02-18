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

package tracker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadata_Keys_ProducesOrderedListOfKeys(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"b": "2",
			"a": "1",
			"c": "3",
		},
	}

	keys := meta.Keys()
	require.Equal(keys, []string{"a", "b", "c"})
}

func TestMetadata_Get_ReturnsValueForKey(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
	}

	require.Equal("value1", meta.Get("key1"))
	require.Equal("value2", meta.Get("key2"))
	require.Nil(meta.Get("key3"))
}

func TestMetadata_String_ProducesOrderedListOfKeys(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"b": "2",
			"a": "1",
			"c": "3",
		},
	}

	result := meta.String()
	expected := "{a: 1, b: 2, c: 3}"
	require.Equal(expected, result)
}
