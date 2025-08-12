<<<<<<<< HEAD:daphne/receiptstore/receipt_store.go
package receiptstore
========
package receipt_store
>>>>>>>> dccf868 (Use ReceiptStore in rpc):daphne/receipt_store/receipt_store.go

import (
	"errors"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

<<<<<<<< HEAD:daphne/receiptstore/receipt_store.go
//go:generate mockgen -source receipt_store.go -destination=receipt_store_mock.go -package=receiptstore
========
//go:generate mockgen -source receipt_store.go -destination=receipt_store_mock.go -package=receipt_store
>>>>>>>> dccf868 (Use ReceiptStore in rpc):daphne/receipt_store/receipt_store.go

// ReceiptStore defines the interface for a receipt store that can ingest transactions
// via blocks and retrieve receipts by transaction hash.
type ReceiptStore interface {
	// Ingests a block and store its transactions and receipts, if any.
	AddBlock(block types.Block) error
	// Retrieves a receipt by its transaction hash.
	// Returns the receipt and a boolean indicating if it was found.
	GetReceipt(txHash types.Hash) (types.Receipt, bool)
}

// NewReceiptStore creates a new instance of ReceiptStore.
func NewReceiptStore() *receiptStore {
	return &receiptStore{}
}

// receiptStore is a simple in-memory implementation of ReceiptStore.
type receiptStore struct {
	receipts map[types.Hash]types.Receipt
}

// AddBlock adds a block's transactions and their corresponding receipts to the store.
func (s *receiptStore) AddBlock(block types.Block) error {
	if s.receipts == nil {
		s.receipts = make(map[types.Hash]types.Receipt)
	}
	if len(block.Transactions) != len(block.Receipts) {
		return errors.New("mismatched number of transactions and receipts in block")
	}
	for i := range len(block.Transactions) {
		s.receipts[block.Transactions[i].Hash()] = block.Receipts[i]
	}
	return nil
}

// GetReceipt retrieves a receipt from the store by its transaction hash.
func (s *receiptStore) GetReceipt(txHash types.Hash) (types.Receipt, bool) {
	receipt, found := s.receipts[txHash]
	return receipt, found
}
