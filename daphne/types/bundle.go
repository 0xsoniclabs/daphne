package types

import (
	"fmt"
	"time"
)

// Bundle represents a verifiable product of consensus.
// It contains a bundle number and a slice of transactions.
type Bundle struct {
	Number       uint32
	Transactions []Transaction
	Timestamp    time.Time
}

func (b *Bundle) String() string {
	return fmt.Sprintf("Bundle%+v", *b)
}
