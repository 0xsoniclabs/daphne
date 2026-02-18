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

package payload

import (
	"bytes"
	"cmp"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Transactions is a simple payload type that holds a list of transactions.
type Transactions []types.Transaction

func (p Transactions) Size() uint32 {
	var size uint32
	for _, tx := range p {
		size += tx.MessageSize()
	}
	return size
}

func (p Transactions) Clone() Payload {
	return slices.Clone(p)
}

// sortTransactionsInExecutionOrder sorts transactions in an order they can be
// executed in. Transactions need to be executed in ascending order of nonce per
// sender. For transactions with the same nonce, we use the hash to ensure a
// deterministic order.
func sortTransactionsInExecutionOrder(txs []types.Transaction) []types.Transaction {
	slices.SortFunc(txs, func(a, b types.Transaction) int {
		r := cmp.Compare(a.Nonce, b.Nonce)
		if r != 0 {
			return r
		}
		hashA := a.Hash()
		hashB := b.Hash()
		return bytes.Compare(hashA[:], hashB[:])
	})
	return txs
}
