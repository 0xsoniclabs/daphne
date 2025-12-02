package payload

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source protocol.go -destination=protocol_mock.go -package=payload

// Protocol defines the methods for building and merging payloads in the DAG
// consensus. It abstracts the process of the block formation, by defining the
// contributions of individual events to the final blocks.
type Protocol[P Payload] interface {
	// BuildPayload constructs a payload for a new event from the given
	// candidate transactions.
	BuildPayload(event EventInfo, lineup *txpool.Lineup) P

	// Merge combines multiple payloads from different events confirmed by the
	// DAG consensus into a list of bundles that are confirmed.
	Merge(payloads []P) []types.Bundle
}

type EventInfo interface {
	GetRound() uint32
}

// ProtocolFactory is a factory for creating new instances of a payload protocol.
type ProtocolFactory[P Payload] interface {
	NewProtocol(
		committee *consensus.Committee,
		localValidatorId consensus.ValidatorId,
	) Protocol[P]

	// String returns a human-readable summary of the protocol's configuration.
	fmt.Stringer
}
