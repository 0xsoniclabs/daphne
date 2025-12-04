package payload

import (
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// RawProtocol is a simple implementation of the Protocol interface that
// directly uses the candidate transactions as the payload and merges them
// by concatenation. It uses [Transactions] as the payload type.
type RawProtocol struct{}

func (p RawProtocol) BuildPayload(candidates txpool.Lineup) Transactions {
	return slices.Clone(candidates.All())
}

func (p RawProtocol) Merge(payloads []Transactions) []types.Bundle {
	var txs []types.Transaction
	for _, payload := range payloads {
		txs = append(txs, payload...)
	}
	return []types.Bundle{{Transactions: txs}}
}

func (p RawProtocol) String() string {
	return "raw"
}
