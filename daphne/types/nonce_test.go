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

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNonce_String_PrintsAsString(t *testing.T) {
	tests := map[Nonce]string{
		0:   "0",
		1:   "1",
		256: "256",
	}

	for nonce, expected := range tests {
		t.Run(fmt.Sprintf("%d", nonce), func(t *testing.T) {
			require.Equal(t, expected, nonce.String())
		})
	}
}

func TestNonce_Serialize_DeserializesAsBigEndian8byteValue(t *testing.T) {
	tests := map[Nonce][]byte{
		1:   {0, 0, 0, 0, 0, 0, 0, 1},
		15:  {0, 0, 0, 0, 0, 0, 0, 15},
		512: {0, 0, 0, 0, 0, 0, 2, 0},
	}

	for nonce, expected := range tests {
		t.Run(fmt.Sprintf("%d", nonce), func(t *testing.T) {
			data := nonce.Serialize()
			require.Equal(t, expected, data)

			var deserialized Nonce
			err := deserialized.Deserialize(data)
			require.NoError(t, err)
			require.Equal(t, nonce, deserialized)
		})
	}
}

func TestNonce_Deserialize_FailsOnWrongInputLength(t *testing.T) {
	tests := [][]byte{
		{0, 0, 0, 0, 0, 0, 0},       // too short
		{0, 0, 0, 0, 0, 0, 1, 2, 3}, // too long
	}

	for _, data := range tests {
		t.Run(fmt.Sprintf("%v", data), func(t *testing.T) {
			var nonce Nonce
			err := nonce.Deserialize(data)
			require.Error(t, err)
		})
	}
}
