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
	"crypto/sha256"
	"fmt"
)

// Hash is a cryptographic digest of a value type.
type Hash [32]byte

// String returns the hexadecimal representation of the Hash.
func (h Hash) String() string {
	return fmt.Sprintf("0x%x", h[:])
}

// Sha256 computes the SHA-256 hash of the input data and returns it as a Hash.
func Sha256(data []byte) Hash {
	return Hash(sha256.Sum256(data))
}
