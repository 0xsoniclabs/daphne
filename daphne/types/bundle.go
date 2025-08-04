package types

import "fmt"

type Bundle struct {
	Number       uint32
	Transactions []Transaction
}

func (b *Bundle) String() string {
	return fmt.Sprintf("Block%+v", *b)
}
