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
	BuildPayload(EventMeta, txpool.Lineup) P

	// Merge combines multiple payloads from different events confirmed by the
	// DAG consensus into a list of bundles that are confirmed.
	Merge(payloads []P) []types.Bundle
}

// EventMeta provides contextual information about the event for which
// the payload is being built by the protocol.
type EventMeta struct {
	// ParentsMaxRound is the maximum round at least one of the event's parents
	// belongs to, 0 if the event has no parents.
	ParentsMaxRound uint32
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
