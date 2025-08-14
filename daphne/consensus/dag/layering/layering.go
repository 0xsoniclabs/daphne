package layering

import "github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"

//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

// Layering is a process of layering events in a DAG. The goal of this process
// is to identify roles for the DAG events, where groups of these roles are in fact layers.
// Forming layers creates basis for breaking DAG-asymmetry which allows for linearization
// of the dag events.
type Layering interface {
	// IsCompatible checks if an event is compatible with the current layering.
	// e.g. number of events, sequencing, fork criteria etc.
	// GetCompatibilityRules(event model.EventMessage) []RuleValidator
	// IsCandidate reports if a DAG event is a viable candidate for a leader role
	// based on its relationships with existing layers.
	IsCandidate(dag *model.Dag, event *model.Event) (bool, error)
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
