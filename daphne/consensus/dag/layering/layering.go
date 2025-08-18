package layering

import (
	"errors"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

type CommonValidator struct {
	Committee map[model.CreatorId]uint32
}

func (i *CommonValidator) Validate(event *model.Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	if _, exists := i.Committee[event.Creator()]; !exists {
		return errors.New("event creator is not in committee")
	}
	// if event.Seq() == 1 && event.SelfParent() != nil {
	// 	return errors.New("event with seq 1 must not have a self parent")
	// }
	// if event.SelfParent() == nil && event.Seq() != 1 {
	// 	return errors.New("event without self parent must have seq 0")
	// }
	// if !event.IsGenesis() && event.Seq() != event.SelfParent().Seq()+1 {
	// 	return errors.New("event sequence must be one greater than its self parent")
	// }
	return nil
}

// Layering is a process of layering events in a DAG. The goal of this process
// is to identify roles for the DAG events, where groups of these roles are in fact layers.
// Forming layers creates basis for breaking DAG-asymmetry which allows for linearization
// of the dag events.
// Layering is a stateless decision-making engine that operates on the DAG structure
// without maintaining any internal state between calls. It has a responsibility
// of validating all passed events of _interest_ while making decisions about event
// roles in the DAG.
type Layering interface {
	// IsCompatible checks if an event is compatible with the current layering.
	// e.g. number of events, sequencing, fork criteria etc.
	Validate(event *model.Event) error
	// IsCandidate reports if a DAG event is a viable candidate for a leader role
	// based on its relationships with existing layers.
	IsCandidate(event *model.Event) (bool, error)
	// IsLeader identifies the event's current leader status based on its
	// relationships with existing layers by returning a current Verdict.
	IsLeader(dag *model.Dag, event *model.Event) (Verdict, error)
	// SortLeaders linearizes a sequence of leaders by a deterministic criteria.
	SortLeaders(events []*model.Event) ([]*model.Event, error)
}

type RuleValidator func(event model.EventMessage) error

type LayeringFactory interface {
	NewLayering(committee map[model.CreatorId]uint32) (Layering, error)
}

// Verdict represents current leader status of a DAG event.
type Verdict int

const (
	// VerdictYes declares an event as a finalized leader.
	VerdictYes Verdict = iota
	// VerdictNo marks an event as out of the running for leader.
	VerdictNo
	// VerdictUndecided indicates that the event's leader status is not yet determined.
	// It may change as new events are added to the DAG.
	VerdictUndecided
)
