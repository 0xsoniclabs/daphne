package payload

import (
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
