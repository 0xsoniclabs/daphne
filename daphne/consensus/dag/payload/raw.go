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

package payload

import (
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// RawProtocol is a simple implementation of the Protocol interface that
// directly uses the candidate transactions as the payload and merges them
// by concatenation. It uses [Transactions] as the payload type.
type RawProtocol struct{}

func (p RawProtocol) BuildPayload(
	_ EventMeta,
	lineup txpool.Lineup,
) Transactions {
	return slices.Clone(lineup.All())
}

func (p RawProtocol) Merge(payloads []Transactions) []types.Bundle {
	var txs []types.Transaction
	for _, payload := range payloads {
		txs = append(txs, payload...)
	}
	sortTransactionsInExecutionOrder(txs)
	return []types.Bundle{{Transactions: txs}}
}

// RawProtocolFactory is a factory for creating instances of RawProtocol.
type RawProtocolFactory struct{}

func (f RawProtocolFactory) NewProtocol(
	*consensus.Committee,
	consensus.ValidatorId,
) Protocol[Transactions] {
	return RawProtocol{}
}

func (f RawProtocolFactory) String() string {
	return "raw"
}
