// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package layering

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

//go:generate mockgen -source layering.go -destination=layering_mock.go -package=layering
//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

// Layering is a process of assigning roles to events in a DAG. Events in
// the same roles which are grouped by a Layering-specific criteria form layers.
// Forming layers creates basis for breaking DAG-asymmetry which allows for
// linearization of the dag events.
// Layering is associated with a single DAG instance.
// Layering is a decision-making engine that makes decisions on the associated DAG.
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
	IsCandidate(event *model.Event) bool
	// IsLeader identifies the event's current leader status by returning a [Verdict].
	// The verdict is based on its relationship with layers identified in the associated DAG.
	// If the event is a leader, [VerdictYes] is returned. If the relationships in the DAG
	// make the event's election as a leader no longer possible, [VerdictNo] is returned.
	// If the event is still eligible for being a leader, i.e. a larger DAG than the
	// current one is required in order to elect the event (or one of its competitors),
	// [VerdictUndecided] is returned.
	IsLeader(event *model.Event) Verdict
	// SortLeaders orders a sequence of leaders by a deterministic criteria.
	// Any non-leader events are filtered out so the resulting slice may contains less elements than the original.
	SortLeaders(events []*model.Event) []*model.Event
	// GetRound extracts a round number from the event according to the
	// layering's criteria. A round should provide a metric of progress in the
	// DAG as perceived by the layering. Rounds are expected to be monotonically
	// increasing as new events are added to the DAG. In particular, no event
	// may have a round number lower than that of any of its parents.
	GetRound(event *model.Event) uint32
}

type Factory interface {
	NewLayering(dag model.Dag, committee *consensus.Committee) Layering
	fmt.Stringer
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
