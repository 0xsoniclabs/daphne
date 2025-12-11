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
