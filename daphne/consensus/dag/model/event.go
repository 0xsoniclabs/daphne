package model

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

type EventId types.Hash

func (c EventId) Serialize() []byte {
	return c[:]
}

func (c EventId) String() string {
	return fmt.Sprintf("%X", c[:8])
}

// Event represents a consensus event in the DAG.
// This is a local representation of an event that resides in memory.
// It contains the following fields:
// - Seq: The sequence number of the event, that orders events made by the same
// validator (Seq is always one greater than self-parent's Seq). Seq of a genesis event is 1.
// - Creator: The ID of the creator of the event, which is a validator node.
// - Parents: A list of parent events, which are the events that this event builds upon.
// Note that the first parent must be the self-parent, which is the parent created by the
// same validator.
// - Payload: The transactions included in the event.
type Event[P payload.Payload] struct {
	id      EventId
	seq     uint32
	creator consensus.ValidatorId
	parents []*Event[P]
	payload P
}

// NewEvent creates a new Event instance. It performs checks to ensure that the
// first parent is the self-parent, and no parent is nil.
func NewEvent[P payload.Payload](
	creator consensus.ValidatorId,
	parents []*Event[P],
	payload P,
) (*Event[P], error) {
	for _, parent := range parents {
		if parent == nil {
			return nil, errors.New("nil parent event found")
		}
	}
	seq := uint32(1)
	if len(parents) > 0 {
		if parents[0].creator != creator {
			return nil, errors.New("first parent must be the self-parent created by the same validator")
		}
		seq = parents[0].seq + 1
	}
	e := &Event[P]{
		seq:     seq,
		creator: creator,
		parents: slices.Clone(parents),
		payload: payload,
	}
	e.id = e.ToEventMessage().EventId()
	return e, nil
}

// Seq is the getter for the sequence number of the event.
func (e *Event[P]) Seq() uint32 {
	return e.seq
}

// Creator is the getter for the creator ID of the event.
func (e *Event[P]) Creator() consensus.ValidatorId {
	return e.creator
}

// Parents returns a copy of the slice of parent events.
func (e *Event[P]) Parents() []*Event[P] {
	return slices.Clone(e.parents)
}

// Payload returns a copy of the slice of transactions included in the event.
func (e *Event[P]) Payload() P {
	return e.payload.Clone().(P)
}

func (e *Event[P]) EventId() EventId {
	return e.id
}

// ToEventMessage converts an Event to a format suitable for
// network transmission.
func (e *Event[P]) ToEventMessage() EventMessage[P] {
	parents := []EventId{}
	for _, parent := range e.parents {
		parents = append(parents, parent.EventId())
	}
	return EventMessage[P]{
		Creator: e.creator,
		Parents: parents,
		Payload: e.payload.Clone().(P),
	}
}

// SelfParent returns the parent of the event that has the same creator.
// If there are no parents, it returns nil.
// Every non-genesis event is expected to have a parent event from the same creator,
// at index 0 of the Parents slice.
func (e *Event[P]) SelfParent() *Event[P] {
	if len(e.parents) > 0 {
		return e.parents[0]
	}
	return nil
}

// IsGenesis checks if the event is a genesis event, which has no parents.
func (e Event[P]) IsGenesis() bool {
	return len(e.parents) == 0
}

// TraverseClosure traverses the closure of the event with a simple depth-first
// search, calling the provided visitor method on each event. The closure of an
// event includes the event itself and all its parents recursively (all ancestors).
// The traversal is controlled by the result returned from [EventVisitor.Visit].
// It can indicate to continue descending, prune the current branch, or abort
// the entire traversal.
// The visitor can be used to perform operations with each event while filtering
// out certain paths based on custom logic for the sake of performance.
func (e *Event[P]) TraverseClosure(visitor EventVisitor[P]) {
	visited := sets.Empty[*Event[P]]()
	abort := false
	var traverse func(*Event[P])
	traverse = func(event *Event[P]) {
		if abort {
			return
		}
		if visited.Contains(event) {
			return
		}
		visited.Add(event)

		switch visitor.Visit(event) {
		case Visit_Abort:
			abort = true
			return
		case Visit_Prune:
			return
		case Visit_Descent:
			// Continue the traversal.
			for _, parent := range event.parents {
				traverse(parent)
			}
		}
	}
	traverse(e)
}

// EventMessage represents a network message containing event data,
// used to transmit events across the network. This structure is needed
// because the Event structure contains pointers to other events,
// which cannot be serialized directly for network transmission.
type EventMessage[P payload.Payload] struct {
	Creator consensus.ValidatorId
	Parents []EventId
	Payload P
}

func (e EventMessage[P]) EventId() EventId {
	data := []byte{}
	data = append(data, e.Creator.Serialize()...)
	for _, parent := range e.Parents {
		data = append(data, parent.Serialize()...)
	}
	return EventId(types.Sha256(data))
}

func (e EventMessage[P]) MessageSize() uint32 {
	res := uint32(reflect.TypeFor[EventMessage[P]]().Size()) +
		uint32(len(e.Parents))*uint32(reflect.TypeFor[EventId]().Size())
	res += e.Payload.Size()
	return res
}
