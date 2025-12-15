package payload

import (
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// RawProtocol is a simple implementation of the Protocol interface that
// directly uses the candidate transactions as the payload and merges them
// by concatenation. It uses [Transactions] as the payload type.
type RawProtocol struct{}

func (p RawProtocol) OnConnectedEventPayload(consensus.ValidatorId, uint32, Transactions) {
	// No-op
}

func (p RawProtocol) BuildPayload(
	_ EventMeta[Transactions],
	provider consensus.TransactionProvider,
) Transactions {
	return slices.Clone(provider.GetCandidateLineup().All())
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
