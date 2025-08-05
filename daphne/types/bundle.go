package types

import "fmt"

// Bundle represents a verifiable product of consensus.
// It contains a bundle number and a slice of transactions.
type Bundle struct {
	Number       uint32
	Transactions []Transaction
}

func (b *Bundle) String() string {
	return fmt.Sprintf("Bundle%+v", *b)
}
