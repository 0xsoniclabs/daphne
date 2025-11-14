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
	return 128 // 128 is the average size of a real transaction in bytes.
}
