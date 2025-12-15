package types

import (
	"fmt"
	"slices"
)

// Bundle represents a verifiable product of consensus.
// It contains a bundle number and a slice of transactions.
type Bundle struct {
	Number       uint32
	Transactions []Transaction
}

func (b *Bundle) String() string {
	return fmt.Sprintf("Bundle%+v", *b)
}

func (b *Bundle) Size() uint32 {
	size := uint32(4) // Size of Number
	for _, tx := range b.Transactions {
		size += tx.MessageSize()
	}
	return size
}

func (b *Bundle) Clone() *Bundle {
	return &Bundle{
		Number:       b.Number,
		Transactions: slices.Clone(b.Transactions),
	}
}
