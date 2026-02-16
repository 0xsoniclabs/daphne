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

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddress_String_PrintsWithPrefix(t *testing.T) {
	tests := map[Address]string{
		0:   "#0",
		1:   "#1",
		256: "#256",
	}

	for addr, expected := range tests {
		t.Run(fmt.Sprintf("%d", addr), func(t *testing.T) {
			require.Equal(t, expected, addr.String())
		})
	}
}

func TestAddress_Serialize_DeserializesAsBigEndian8byteValue(t *testing.T) {
	tests := map[Address][]byte{
		0:   {0, 0, 0, 0, 0, 0, 0, 0},
		1:   {0, 0, 0, 0, 0, 0, 0, 1},
		256: {0, 0, 0, 0, 0, 0, 1, 0},
	}

	for addr, expected := range tests {
		t.Run(fmt.Sprintf("%d", addr), func(t *testing.T) {
			data := addr.Serialize()
			require.Equal(t, expected, data)

			var deserialized Address
			err := deserialized.Deserialize(data)
			require.NoError(t, err)
			require.Equal(t, addr, deserialized)
		})
	}
}

func TestAddress_Deserialize_FailsOnWrongInputLength(t *testing.T) {
	tests := [][]byte{
		{0, 0, 0, 0, 0, 0, 0},       // too short
		{0, 0, 0, 0, 0, 0, 0, 1, 2}, // too long
	}

	for _, data := range tests {
		t.Run(fmt.Sprintf("%v", data), func(t *testing.T) {
			var addr Address
			err := addr.Deserialize(data)
			require.Error(t, err)
		})
	}
}
