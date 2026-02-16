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
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// ProactiveEmitterFactory is a factory for creating [ProactiveEmitter] instances.
// ProactiveEmitter emits new events when it detects a NumNewParents number of new events,
// compared to the last emission.
// It also ensures that there is no double emission for the same event sequence number.
type ProactiveEmitterFactory struct {
	NumNewParents int
}

type ProactiveEmitter struct {
	dag     model.Dag
	creator consensus.ValidatorId
	channel Channel

	// lastEmittedSeq represents the sequence number of the last emitted event by the emitter.
	// It is incremented upon each successful emission.
	lastEmittedSeq uint32
	// lastEmittedParents holds the parents of the last emitted event by the emitter.
	lastEmittedParents map[consensus.ValidatorId]*model.Event
	numNewParents      int

	stateMutex sync.Mutex
}

func (f ProactiveEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId, layering layering.Layering) Emitter {
	return newProactiveEmitter(channel, dag, creator, f.NumNewParents, layering)
}

func newProactiveEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId, numNewParents int, _ layering.Layering) *ProactiveEmitter {
	return &ProactiveEmitter{
		dag:           dag,
		creator:       creator,
		channel:       channel,
		numNewParents: numNewParents,
	}
}

func (f ProactiveEmitterFactory) String() string {
	return fmt.Sprintf("proactive_%d", f.NumNewParents)
}

func (e *ProactiveEmitter) OnChange() {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	if e.channel == nil {
		return
	}

	dagHeads := e.dag.GetHeads()
	if e.shouldEmit(dagHeads) {
		e.channel.Emit(dagHeads)

		e.lastEmittedParents = dagHeads
		e.lastEmittedSeq++
	}
}

func (e *ProactiveEmitter) shouldEmit(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	if e.lastEmittedSeq == 0 {
		// Always emit the genesis event
		return true
	}

	lastObservedEvent, observedGenesisEmission := dagHeads[e.creator]
	if !observedGenesisEmission {
		return false
	}

	if lastObservedEvent.Seq() < e.lastEmittedSeq {
		return false
	}

	prev := sets.New(slices.Collect(maps.Values(e.lastEmittedParents))...)
	current := sets.New(slices.Collect(maps.Values(dagHeads))...)
	diff := sets.Difference(current, prev)
	return diff.Size() >= e.numNewParents
}

func (e *ProactiveEmitter) Stop() {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	e.channel = nil
}
