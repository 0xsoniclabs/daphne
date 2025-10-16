package model

import (
	"errors"
	"fmt"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

//go:generate mockgen -source event.go -destination=event_mock.go -package=model

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
type Event struct {
	id      EventId
	seq     uint32
	creator consensus.ValidatorId
	parents []*Event
	payload []types.Transaction
}

// NewEvent creates a new Event instance. It performs checks to ensure that the
// first parent is the self-parent, and no parent is nil.
func NewEvent(creator consensus.ValidatorId, parents []*Event, payload []types.Transaction) (*Event, error) {
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
	e := &Event{
		seq:     seq,
		creator: creator,
		parents: slices.Clone(parents),
		payload: slices.Clone(payload),
	}
	e.id = e.ToEventMessage().EventId()
	return e, nil
}

// Seq is the getter for the sequence number of the event.
func (e *Event) Seq() uint32 {
	return e.seq
}

// Creator is the getter for the creator ID of the event.
func (e *Event) Creator() consensus.ValidatorId {
	return e.creator
}

// Parents returns a copy of the slice of parent events.
func (e *Event) Parents() []*Event {
	return slices.Clone(e.parents)
}

// Payload returns a copy of the slice of transactions included in the event.
func (e *Event) Payload() []types.Transaction {
	return slices.Clone(e.payload)
}

func (e *Event) EventId() EventId {
	return e.id
}

// ToEventMessage converts an Event to a format suitable for
// network transmission.
func (e *Event) ToEventMessage() EventMessage {
	parents := []EventId{}
	for _, parent := range e.parents {
		parents = append(parents, parent.EventId())
	}
	return EventMessage{
		Creator: e.creator,
		Parents: parents,
		Payload: e.payload,
	}
}

// SelfParent returns the parent of the event that has the same creator.
// If there are no parents, it returns nil.
// Every non-genesis event is expected to have a parent event from the same creator,
// at index 0 of the Parents slice.
func (e *Event) SelfParent() *Event {
	if len(e.parents) > 0 {
		return e.parents[0]
	}
	return nil
}

// IsGenesis checks if the event is a genesis event, which has no parents.
func (e Event) IsGenesis() bool {
	return len(e.parents) == 0
}

// TraverseClosure traverses the closure of the event with a simple depth-first
// search, calling the provided visitor method on each event. The closure of an
// event includes the event itself and all its parents recursively (all ancestors).
// If [EventVisitor.Visit] returns true, the traversal stops for that branch.
// The visitor can be used to perform operations with each event while filtering
// out certain paths based on custom logic for the sake of performance.
func (e *Event) TraverseClosure(visitor EventVisitor) {
	visited := sets.Empty[*Event]()
	var traverse func(*Event)
	traverse = func(event *Event) {
		if visited.Contains(event) {
			return
		}
		visited.Add(event)

		if visitor.Visit(event) {
			return
		}
		for _, parent := range event.parents {
			traverse(parent)
		}
	}
	traverse(e)
}

// EventVisitor is an interface for visiting events during DAG traversal.
// It allows for custom logic and filtering to be executed on each event visited.
type EventVisitor interface {
	// Visit should be called by the traversal algorithms on each event.
	// If Visit returns true, it signals that further events on this branch
	// are of no interest to the visitor and thus can be skipped, i.e. the
	// branch can be pruned.
	Visit(event *Event) bool
}

// WrapEventVisitor wraps a function with a signature func(event *Event) bool
// into an EventVisitor adapter that can be used in traversal methods.
// This is a convenience function to allow using simple functions as event
// handlers without having to define a new type.
func WrapEventVisitor(f func(*Event) bool) EventVisitor {
	return &eventVisitor{visit: f}
}

type eventVisitor struct {
	visit func(*Event) bool
}

func (v *eventVisitor) Visit(event *Event) bool {
	return v.visit(event)
}

// GetClosure returns the closure of an event, which includes
// the event itself and all its parents recursively (all ancestors).
func (e *Event) GetClosure() map[*Event]struct{} {
	closure := make(map[*Event]struct{})
	var traverse func(*Event)
	traverse = func(event *Event) {
		if _, exists := closure[event]; exists {
			return
		}
		closure[event] = struct{}{}
		for _, parent := range event.parents {
			traverse(parent)
		}
	}
	traverse(e)
	return closure
}

// EventMessage represents a network message containing event data,
// used to transmit events across the network. This structure is needed
// because the Event structure contains pointers to other events,
// which cannot be serialized directly for network transmission.
type EventMessage struct {
	Creator consensus.ValidatorId
	Parents []EventId
	Payload []types.Transaction
}

func (e EventMessage) EventId() EventId {
	data := []byte{}
	data = append(data, e.Creator.Serialize()...)
	for _, parent := range e.Parents {
		data = append(data, parent.Serialize()...)
	}
	return EventId(types.Sha256(data))
}
