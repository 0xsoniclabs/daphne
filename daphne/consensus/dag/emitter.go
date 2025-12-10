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

type Emitter[P payload.Payload] struct {
	conditions         []Cond
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
) *Emitter[P] {
	return &Emitter[P]{
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

	for _, condition := range e.conditions {
		condition.Init()
	}
}

func (e *Emitter[P]) AddConditions(conditions ...Cond) {
	e.igniteMu.Lock()
	defer e.igniteMu.Unlock()

	e.conditions = append(e.conditions, conditions...)
}

func (e *Emitter[P]) isEmissionReady() bool {
	if e.conditions == nil {
		return false
	}
	for _, condition := range e.conditions {
		if !condition.Evaluate() {
			return false
		}
	}
	return true
}

func (e *Emitter[P]) AttemptEmission() {
	e.mu.Lock()
	defer e.mu.Unlock()

	fmt.Print("Attempting emission, head state: ", e.dag.GetHeads(), " - ")

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

	fmt.Println("emitting, new seq: ", e.lastEmittedSeq)

	e.channel.Broadcast(makeEventMessage(
		e.creator,
		parents,
		e.payloads.BuildPayload(e.transactionProvider.GetCandidateLineup()),
	))

	e.Ignite()
}

func (e *Emitter[P]) Stop() {
	e.igniteMu.Lock()
	defer e.igniteMu.Unlock()
	e.conditions = nil
}

type Cond interface {
	Init()
	Evaluate() bool
}

type ObservesNewParentsCondition[P payload.Payload] struct {
	numParents int
	emitter    *Emitter[P]
}

func (onpc *ObservesNewParentsCondition[P]) Init() {
}

func (onpc *ObservesNewParentsCondition[P]) Evaluate() bool {
	dagHeads := onpc.emitter.dag.GetHeads()
	_, emittedAtLeastOnce := dagHeads[onpc.emitter.creator]
	if !emittedAtLeastOnce {
		return true
	}

	prev := sets.New(slices.Collect(maps.Values(onpc.emitter.lastEmittedParents))...)
	new := sets.New(slices.Collect(maps.Values(dagHeads))...)
	diff := sets.Difference(new, prev)

	// fmt.Println("Emitter for ", e.creator, " latest seq ", e.lastEmittedSeq, ", will emit ", diff.Size() >= numParents)
	return diff.Size() >= onpc.numParents
}

type timeoutCondition[P payload.Payload] struct {
	duration time.Duration
	done     atomic.Bool
	emitter  *Emitter[P]
}

func (tc *timeoutCondition[P]) Init() {
	tc.done = atomic.Bool{}
	go func() {
		time.Sleep(tc.duration)
		tc.done.Store(true)

		tc.emitter.AttemptEmission()
	}()
}

func (tc *timeoutCondition[P]) Evaluate() bool {
	return tc.done.Load()
}

type ObservesLatestEmissionCondition[P payload.Payload] struct {
	emitter *Emitter[P]
}

func (olec *ObservesLatestEmissionCondition[P]) Init() {
}

func (olec *ObservesLatestEmissionCondition[P]) Evaluate() bool {
	dagHeads := olec.emitter.dag.GetHeads()
	lastSeenEvent, emittedAtLeastOnce := dagHeads[olec.emitter.creator]

	latestObservedSeq := uint32(0)
	if emittedAtLeastOnce {
		latestObservedSeq = lastSeenEvent.Seq()
	}

	// fmt.Println("Emitter for ", e.creator, " latest seq ", e.lastEmittedSeq, ", will emit ", latestObservedSeq >= e.lastEmittedSeq)
	return latestObservedSeq >= olec.emitter.lastEmittedSeq
}
