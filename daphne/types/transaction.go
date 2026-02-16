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

// Transaction represents a transfer of coins between two Daphne accounts.
// Nonce is used to prevent replay attacks and to order transactions
// from a same sender.
type Transaction struct {
	From  Address
	To    Address
	Value Coin
	Nonce Nonce
}

func (t *Transaction) String() string {
	return fmt.Sprintf(
		"Tx{From: %s, To: %s, Value: %s, Nonce: %s}",
		t.From.String(), t.To.String(), t.Value.String(), t.Nonce.String(),
	)
}

func (t *Transaction) Hash() Hash {
	bytes := make([]byte, 0, 8+8+8+8)
	bytes = append(bytes, t.From.Serialize()...)
	bytes = append(bytes, t.To.Serialize()...)
	bytes = append(bytes, t.Value.Serialize()...)
	bytes = append(bytes, t.Nonce.Serialize()...)
	return Hash(sha256.Sum256(bytes))
}

func (t Transaction) MessageSize() uint32 {
	return 128 // TODO: replace by measured average transaction size in bytes.
}
