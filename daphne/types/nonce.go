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
	"encoding/binary"
	"fmt"
)

// Nonce is used to track the number of transactions sent from an account in
// the chain emulated by Daphne.
type Nonce uint64

func (n Nonce) String() string {
	return fmt.Sprintf("%d", n)
}

// Serialize encodes the Nonce as a big-endian 8-byte value.
func (n *Nonce) Serialize() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(*n))
}

// Deserialize decodes a big-endian 8-byte value into a Nonce.
// It returns an error if the input data is not exactly 8 bytes long.
func (n *Nonce) Deserialize(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("data must be exactly 8 bytes to deserialize Nonce")
	}
	*n = Nonce(binary.BigEndian.Uint64(data[:8]))
	return nil
}
