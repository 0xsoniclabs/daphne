package store

import (
	"log/slog"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

type Store struct {
	receipts map[types.Hash]types.Receipt
}

func (s *Store) AddBlock(block types.Block) {
	if s.receipts == nil {
		s.receipts = make(map[types.Hash]types.Receipt)
	}
	if len(block.Transactions) != len(block.Receipts) {
		slog.Error("Received block with mismatched transactions and receipts")
	}
	for i := range len(block.Transactions) {
		s.receipts[block.Transactions[i].Hash()] = block.Receipts[i]
	}
}

func (s *Store) GetReceipt(hash types.Hash) (types.Receipt, bool) {
	receipt, found := s.receipts[hash]
	return receipt, found
}
