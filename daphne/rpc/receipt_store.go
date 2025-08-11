package rpc

import (
	"errors"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

// receiptStore is a simple in-memory store for transaction receipts. It
// maps transaction hashes to their corresponding receipts.
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
