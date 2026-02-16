// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package receiptstore

import (
	"errors"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source receipt_store.go -destination=receipt_store_mock.go -package=receiptstore

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
