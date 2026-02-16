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

func TestHash_String_Produces32ByteHexOutput(t *testing.T) {
	tests := map[Hash]string{
		{}:     "0x0000000000000000000000000000000000000000000000000000000000000000",
		{1, 2}: "0x0102000000000000000000000000000000000000000000000000000000000000",
	}

	for input, expected := range tests {
		t.Run(fmt.Sprintf("%v", input), func(t *testing.T) {
			require.Equal(t, expected, input.String())
		})
	}
}

func TestSha256_TestKnownHashes(t *testing.T) {
	tests := map[string]struct {
		input    []byte
		expected string
	}{
		"nil": {
			input:    nil,
			expected: "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		"empty": {
			input:    []byte{},
			expected: "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		"non-empty": {
			input:    []byte("hello world"),
			expected: "0xb94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := Sha256(test.input)
			require.Equal(t, test.expected, result.String())
		})
	}
}
