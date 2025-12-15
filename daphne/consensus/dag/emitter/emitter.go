package emitter

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
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
	NewEmitter(Channel, model.Dag, consensus.ValidatorId) Emitter
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
