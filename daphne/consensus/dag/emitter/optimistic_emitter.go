package emitter

import (
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

type OptimisticEmitterFactory struct {
	NumNewParents int
}

type OptimisticEmitter struct {
	dag     model.Dag
	creator consensus.ValidatorId
	channel Channel

	lastEmittedSeq   uint32
	lastEmittedHeads map[consensus.ValidatorId]*model.Event
	NumNewParents    int

	mu sync.Mutex
}

func (f *OptimisticEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId) *OptimisticEmitter {
	return &OptimisticEmitter{
		dag:           dag,
		creator:       creator,
		channel:       channel,
		NumNewParents: f.NumNewParents,
	}
}

func (e *OptimisticEmitter) OnDagChange() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel == nil {
		fmt.Println("Stopped")
		return
	}

	dagHeads := e.dag.GetHeads()
	if e.shouldEmit(dagHeads) {
		e.lastEmittedHeads = dagHeads
		e.lastEmittedSeq++
		e.channel.Emit(dagHeads)
		fmt.Println("Emitting")
	} else {
		fmt.Println("Not emitting")
	}
}

func (e *OptimisticEmitter) shouldEmit(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	lastObservedEvent, emittedAtLeastOnce := dagHeads[e.creator]

	lastObservedEventSeq := uint32(0)
	if !emittedAtLeastOnce {
		return true
	} else {
		lastObservedEventSeq = lastObservedEvent.Seq()
	}

	if lastObservedEventSeq < e.lastEmittedSeq {
		return false
	}

	prev := sets.New(slices.Collect(maps.Values(e.lastEmittedHeads))...)
	current := sets.New(slices.Collect(maps.Values(dagHeads))...)
	diff := sets.Difference(current, prev)

	return diff.Size() >= e.NumNewParents
}

func (e *OptimisticEmitter) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.channel = nil
}
