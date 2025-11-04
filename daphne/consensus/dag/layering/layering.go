package layering

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate mockgen -source layering.go -destination=layering_mock.go -package=layering
//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

// Layering is a process of assigning roles to events in a DAG. Events in
// the same roles which are grouped by a Layering-specific criteria form layers.
// Forming layers creates basis for breaking DAG-asymmetry which allows for
// linearization of the dag events.
// Layering is a stateless decision-making engine that makes independent decisions
// on the provided DAG.
// Layering may contain caches and can't be assumed to be thread-safe.
//
// Events can have the following roles:
//   - a "Candidate" is an event that may at some point be elected as a leader. Candidate
//     status is a static property of an event and remains constant over time
//   - a "Leader" is a Candidate event that got promoted through an election to a leader.
//     Every candidate's leader role is initially undecided, and may eventually be confirmed
//     or rejected. Until then, the leader role of a candidate remains "undecided". Once
//     decided, the decision is permanent (the leader decision is monotonic).
type Layering interface {
	// IsCandidate reports if an event is a viable candidate for a leader
	// role based only on its relationship with observed layers.
	IsCandidate(dag *model.Dag, event *model.Event) bool
	// IsLeader identifies the event's current leader status by returning a [Verdict].
	// The verdict is solely based on its relationship with layers identified in the provided dag.
	// If the event is a leader, [VerdictYes] is returned. If the relationships in the provided DAG
	// make the event's election as a leader no longer possible, [VerdictNo] is returned.
	// If the event is still eligible for being a leader, i.e. a larger DAG than the
	// one provided is required in order to elect the event (or one of its competitors),
	// [VerdictUndecided] is returned.
	IsLeader(dag *model.Dag, event *model.Event) Verdict
	// SortLeaders orders a sequence of leaders by a deterministic criteria.
	// Any non-leader events are filtered out so the resulting slice may contains less elements than the original.
	SortLeaders(dag *model.Dag, events []*model.Event) []*model.Event
}

type Factory interface {
	NewLayering(committee *consensus.Committee) Layering
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
