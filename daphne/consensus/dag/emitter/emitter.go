package emitter

import (
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate mockgen -source emitter.go -destination=emitter_mock.go -package=emitter

// StartNewEmitter initializes and starts a new Emitter instance.
// It sets up the emitter with the provided creator ID, DAG reference,
// emission channel, and emission condition.
// The emission triggers are done in cycles. At each time only one cycle
// can be active. Triggering an emission during an active cycle
// resets the condition and starts a new cycle.
// The condition is reset upon initialization starting the first cycle.
func StartNewEmitter(
	creator consensus.ValidatorId,
	dag model.Dag,
	emitChannel Channel,
	condition Condition,
) *Emitter {
	e := &Emitter{
		dag:       dag,
		creator:   creator,
		channel:   emitChannel,
		condition: condition,
	}
	e.emissionMutex.Lock()
	condition.Reset(e, make(map[consensus.ValidatorId]*model.Event))
	e.emissionMutex.Unlock()
	return e
}

// Channel defines the abstraction for signaling emission of new events.
// It hides the underlying mechanisms of payload formation and network gossiping.
type Channel interface {
	Emit(parents []model.EventId)
}

// Emitter is responsible for triggering event emissions.
// It evaluates a set condition and determines when to emit new events.
type Emitter struct {
	condition     Condition
	emissionMutex sync.Mutex

	creator consensus.ValidatorId
	dag     model.Dag

	channel Channel
}

// AttemptEmission evaluates the emitter's condition in the active cycle
// and sends an emission signal to the registered channel if the condition is met.
// If the emission is triggered, the condition is reset for the next cycle.
// The method should be called by the consumer (Consensus or a [Condition] itself)
// of the emitter whenever it believes an emission attempt is possible,
// e.g., upon timeout or new DAG updates.
func (e *Emitter) AttemptEmission() {
	e.emissionMutex.Lock()
	defer e.emissionMutex.Unlock()

	if !e.condition.Evaluate(e) {
		return
	}

	dagHeads := e.dag.GetHeads()
	e.channel.Emit(e.chooseParents(dagHeads))

	e.condition.Reset(e, dagHeads)
}

// Stop halts the emitter's operation by replacing its condition with a false condition.
// This effectively makes current cycle unable to trigger any emissions and start
// new cycles.
func (e *Emitter) Stop() {
	e.emissionMutex.Lock()
	defer e.emissionMutex.Unlock()

	e.condition.Stop()
	e.condition = NewFalseCondition()
}

func (e *Emitter) chooseParents(dagHeads map[consensus.ValidatorId]*model.Event) []model.EventId {
	parents := []model.EventId{}
	if _, found := dagHeads[e.creator]; found {
		parents = []model.EventId{dagHeads[e.creator].EventId()}
		for creator, tip := range dagHeads {
			if creator != e.creator {
				parents = append(parents, tip.EventId())
			}
		}
	}
	return parents
}

func (e *Emitter) getCreator() consensus.ValidatorId {
	return e.creator
}

func (e *Emitter) getDag() model.Dag {
	return e.dag
}
