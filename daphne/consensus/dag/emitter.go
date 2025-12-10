package dag

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

type condition func() bool

type conditionGenerator[P payload.Payload] func(*Emitter[P]) condition

type Emitter[P payload.Payload] struct {
	generators         []conditionGenerator[P]
	conditions         []condition
	creator            consensus.ValidatorId
	dag                model.Dag
	lastEmittedSeq     uint32
	lastEmittedParents map[consensus.ValidatorId]*model.Event

	channel             broadcast.Channel[EventMessage[P]]
	payloads            payload.Protocol[P]
	transactionProvider consensus.TransactionProvider

	mu       sync.Mutex
	igniteMu sync.Mutex
}

func NewEmitter[P payload.Payload](
	creator consensus.ValidatorId,
	dag model.Dag,
	payloads payload.Protocol[P],
	transactionProvider consensus.TransactionProvider,
	channel broadcast.Channel[EventMessage[P]],
	generators []conditionGenerator[P],
) *Emitter[P] {
	return &Emitter[P]{
		generators:          generators,
		dag:                 dag,
		creator:             creator,
		lastEmittedParents:  make(map[consensus.ValidatorId]*model.Event),
		payloads:            payloads,
		transactionProvider: transactionProvider,
		channel:             channel,
	}
}

func (e *Emitter[P]) Ignite() {
	e.igniteMu.Lock()
	defer e.igniteMu.Unlock()

	e.conditions = nil
	for _, generator := range e.generators {
		e.conditions = append(e.conditions, generator(e))
	}
}

func (e *Emitter[P]) isEmissionReady() bool {
	for _, condition := range e.conditions {
		if !condition() {
			return false
		}
	}
	return true
}

func (e *Emitter[P]) AttemptEmission() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// fmt.Print("Attempting emission, head state: ", e.dag.GetHeads(), " - ")

	if !e.isEmissionReady() {
		fmt.Print("\n")
		return
	}

	// Emit
	dagHeads := e.dag.GetHeads()
	parents := []model.EventId{}
	if _, found := dagHeads[e.creator]; found {
		parents = []model.EventId{dagHeads[e.creator].EventId()}
		for creator, tip := range dagHeads {
			if creator != e.creator {
				parents = append(parents, tip.EventId())
			}
		}
	}
	e.lastEmittedParents = dagHeads
	e.lastEmittedSeq++

	e.channel.Broadcast(makeEventMessage(
		e.creator,
		parents,
		e.payloads.BuildPayload(e.transactionProvider.GetCandidateLineup()),
	))

	// fmt.Println("emitting, new seq: ", e.lastEmittedSeq)

	e.Ignite()
}

func (e *Emitter[P]) ObservesLatestEmission() condition {
	return func() bool {
		dagHeads := e.dag.GetHeads()
		lastSeenEvent, emittedAtLeastOnce := dagHeads[e.creator]

		latestObservedSeq := uint32(0)
		if emittedAtLeastOnce {
			latestObservedSeq = lastSeenEvent.Seq()
		}

		// fmt.Println("Emitter for ", e.creator, " latest seq ", e.lastEmittedSeq, ", will emit ", latestObservedSeq >= e.lastEmittedSeq)
		return latestObservedSeq >= e.lastEmittedSeq
	}
}

func (e *Emitter[P]) ObservesNewParents(numParents int) condition {
	return func() bool {
		dagHeads := e.dag.GetHeads()
		_, emittedAtLeastOnce := dagHeads[e.creator]
		if !emittedAtLeastOnce {
			return true
		}

		prev := sets.New(slices.Collect(maps.Values(e.lastEmittedParents))...)
		new := sets.New(slices.Collect(maps.Values(dagHeads))...)
		diff := sets.Difference(new, prev)

		// fmt.Println("Emitter for ", e.creator, " latest seq ", e.lastEmittedSeq, ", will emit ", diff.Size() >= numParents)
		return diff.Size() >= numParents
	}
}

func (e *Emitter[P]) TimeoutOccured(duration time.Duration) condition {
	done := atomic.Bool{}
	// fmt.Println("Scheduling timeout for ", duration, ", for creator ", e.creator)
	go func() {
		time.Sleep(duration)
		done.Store(true)

		// fmt.Println("Timeout done for creator ", e.creator)
		e.AttemptEmission()
	}()
	return func() bool {
		return done.Load()
	}
}

func (e *Emitter[P]) Stop() {
	e.igniteMu.Lock()
	defer e.igniteMu.Unlock()
	e.generators = nil
}
