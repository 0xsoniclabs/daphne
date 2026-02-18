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

package emitter

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate mockgen -source emitter.go -destination=emitter_mock.go -package=emitter

// Channel defines the abstraction for signaling emission of new events.
// It hides the underlying mechanisms of payload formation and network gossiping.
type Channel interface {
	// Emit signals the emission of a new event with the parent events
	// on which the new event will be based.
	Emit(parents map[consensus.ValidatorId]*model.Event)
}

// Factory is responsible for parametrizing specific Emitter instances.
type Factory interface {
	// NewEmitter creates a new Emitter instance associated with the given DAG,
	// creator validator ID, and communication channel.
	// If the emission condition of the specific Emitter implementation is based on
	// timers or background processes, those should be started within this method.
	NewEmitter(Channel, model.Dag, consensus.ValidatorId, layering.Layering) Emitter
	String() string
}

// Emitter is a component responsible for triggering event emissions.
// Each implementation evaluates its own conditions for when to emit new events.
type Emitter interface {
	// OnChange notifies the emitter of potential changes in the evaluation state.
	OnChange()
	// Stop terminates any background processes associated with the emitter,
	// and makes all future calls to OnChange no-ops.
	Stop()
}
