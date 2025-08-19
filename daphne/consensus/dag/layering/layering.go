package layering

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

// Layering is a process of assigning roles to events in a DAG. Events in
// the same roles which are grouped by a Layering-specific criteria form layers.
// Forming layers creates basis for breaking DAG-asymmetry which allows for
// linearization of the dag events.
// Layering is a stateless decision-making engine that makes independent decisions
// on the provided DAG. It has a responsibility of validating all decision
// relevant events for the purpose of assigning roles in the DAG.
type Layering interface {
	// Validate checks if an event is valid within the layering context.
	// It ensures that the event meets all the necessary criteria to be considered
	// for layering, i.e. being a candidate, a leader etc.
	// Validate primarily verifies event's self-contained validity only considering
	// its own fields and the relation with its immediate neighbors in the DAG.
	// When performing role assignment throughout the DAG, Validate should be
	// called at each decision relevant event to ensure the validity of its contribution.
	Validate(event *model.Event) error
	// IsCandidate reports if an event is a viable candidate for a leader
	// role based only on its relationship with observed layers.
	// If the provided event, or any event that contributes to event's status of
	// being a candidate, is not valid, an error is returned.
	IsCandidate(event *model.Event) (bool, error)
	// IsLeader identifies the event's current leader status based on its relationships
	// with layers identified in the provided dag, by returning a current Verdict.
	// If the event is a leader, [VerdictYes] is returned. If the conditions in the provided DAG,
	// make the event's election as a leader no longer possible, [VerdictNo] is returned.
	// If the event is still eligible for being a leader, i.e. a larger DAG than the
	// one provided is required in order to elect the event (or one of its competitors),
	// [VerdictUndecided] is returned.
	// If an invalid event is passed, an error is returned.
	IsLeader(dag *model.Dag, event *model.Event) (Verdict, error)
	// SortLeaders orders a sequence of leaders by a deterministic criteria.
	// If non-leader event is passed, an error is returned.
	SortLeaders(events []*model.Event) ([]*model.Event, error)
}

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
