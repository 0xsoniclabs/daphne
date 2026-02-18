// Copyright 2026 Sonic Operations Ltd
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

package rpc

import (
	"github.com/0xsoniclabs/daphne/daphne/receiptstore"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Server provides an end-user interface for customers to interact with the chain.
type Server interface {
	// Send requests the given transaction to be queued for execution on the
	// chain. Send will fail if the the receiving node determines that it can
	// not be processed. For instance, if the used nonce has been used before.
	Send(tx types.Transaction) error

	// IsPending checks whether the local node knows of the transaction as not
	// yet being included in a block.
	IsPending(hash types.Hash) bool

	// GetReceipt checks whether the local node has a receipt for the given
	// transaction hash. If the transaction is not known, it returns false.
	GetReceipt(hash types.Hash) (types.Receipt, bool)
}

// NewServer creates a new RPC server that allows users to interact with a node.
func NewServer(
	pool txpool.TxPool,
	store receiptstore.ReceiptStore,
	tracker tracker.Tracker,
) *server {
	return &server{
		pool:    pool,
		store:   store,
		tracker: tracker,
	}
}

type server struct {
	pool    txpool.TxPool
	store   receiptstore.ReceiptStore
	tracker tracker.Tracker
}

func (s *server) Send(tx types.Transaction) error {
	if s.tracker != nil {
		s.tracker.Track(mark.TxSubmitted, "hash", tx.Hash())
	}
	return s.pool.Add(tx)
}

func (s *server) IsPending(hash types.Hash) bool {
	return s.pool.Contains(hash)
}

func (s *server) GetReceipt(hash types.Hash) (types.Receipt, bool) {
	return s.store.GetReceipt(hash)
}
