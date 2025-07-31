package types

import "fmt"

// Block is a an enumerated block of transactions and their receipts,
// performed on the Daphne.
type Block struct {
	Number       uint32
	Transactions []Transaction
	Receipts     []Receipt
}

func (b *Block) String() string {
	return fmt.Sprintf("Block%+v", *b)
}
