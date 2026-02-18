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

// Copyright 2026 Sonic Labs
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

package autocracy

import (
	"fmt"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

const (
	// DefaultCandidateFrequency is the default frequency for considering events
	// for candidacy if one is not specified in the configuration.
	DefaultCandidateFrequency = uint32(3)
)

// Factory implements [consensus.Factory] and is used to configure
// and produce [Autocracy] layering instances.
// CandidateFrequency parameter controls the frequency at which events are considered
// for candidacy, starting from the genesis event as the first candidate.
type Factory struct {
	CandidateFrequency uint32
}

// NewLayering creates a new [Autocracy] layering instance configured by the factory.
// and associated with the provided creator committee.
func (af Factory) NewLayering(
	dag model.Dag,
	committee *consensus.Committee,
) layering.Layering {
	return newAutocracy(dag, committee, af.CandidateFrequency)
}

// String returns a human-readable summary of the factory configuration.
func (af Factory) String() string {
	frequency := af.CandidateFrequency
	if frequency == 0 {
		frequency = DefaultCandidateFrequency
	}
	return fmt.Sprintf("autocracy-freq=%d", frequency)
}

// Autocracy is a simple testing layering that makes it look like all creators are
// considered, i.e. periodic events by all creators are declared as candidates,
// but in the end the events of the same creator, the autocrat, are always chosen.
// Autocrat is defined as the creator with the lowest ID in the associated committee.
// The Autocracy layers the DAG by identifying candidates as periodic events, with
// a configurable period.
// A candidate is every candidateFrequency-th event created by the same creator.
// A leader is every candidate event created by the committee autocrat, seen by at least
// one different auatocrat's candidate event.
// This leader election policity is not resilient against corruption and should thus,
// never be used in a real world environment.
type Autocracy struct {
	dag                model.Dag
	committee          *consensus.Committee
	autocrat           consensus.ValidatorId
	candidateFrequency uint32
	roundsCache        map[model.EventId]uint32
}

func newAutocracy(
	dag model.Dag,
	committee *consensus.Committee,
	candidateFrequency uint32,
) *Autocracy {
	if candidateFrequency == 0 {
		candidateFrequency = DefaultCandidateFrequency
	}
	return &Autocracy{
		dag:                dag,
		committee:          committee,
		autocrat:           slices.Min(committee.Validators()),
		candidateFrequency: candidateFrequency,
		roundsCache:        make(map[model.EventId]uint32),
	}
}

// IsCandidate returns true for periodic events created by any committee member.
func (a *Autocracy) IsCandidate(event *model.Event) bool {
	// Unprocessable events are considered non-candidates.
	if event == nil || !slices.Contains(a.committee.Validators(), event.Creator()) {
		return false
	}
	return event.Seq()%a.candidateFrequency == 1
}

// IsLeader declares every autocrat's candidate event seen by by at least one
// different autocrat candidate event as a leader. If there is no such successor
// autocrat candidate in the provided DAG, [layering.VerdictUndecided] is returned.
// All other events are reported as not being leaders.
func (a *Autocracy) IsLeader(event *model.Event) layering.Verdict {
	if !a.IsCandidate(event) || a.autocrat != event.Creator() {
		return layering.VerdictNo
	}

	// Check if there is a successor autocrat candidate that reaches this event.
	youngestAutocrat, exists := a.dag.GetHeads()[a.autocrat]
	if !exists {
		return layering.VerdictUndecided
	}
	// Find the youngest autocrat candidate.
	for !a.IsCandidate(youngestAutocrat) {
		youngestAutocrat = youngestAutocrat.SelfParent()
	}

	isReachableByYoungerCandidate := false

	youngestAutocrat.TraverseClosure(
		model.WrapEventVisitor(
			func(traversedEvent *model.Event) model.VisitResult {
				if traversedEvent == event {
					isReachableByYoungerCandidate = true
					return model.Visit_Abort
				}
				return model.Visit_Descent
			}),
	)

	// If the youngest autocrat candidate is the event candidate itself,
	// the candidate does not fulfill the condition of being a leader yet.
	if youngestAutocrat != event && isReachableByYoungerCandidate {
		return layering.VerdictYes
	}

	return layering.VerdictUndecided
}

// SortLeaders verifies the leader status of the passed events and given the simple
// periodicity election, returns them sorted by their sequence number.
// Any non-leaders are filtered out.
func (a *Autocracy) SortLeaders(events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return a.IsLeader(event) != layering.VerdictYes
	})
	slices.SortFunc(leaders, func(l, r *model.Event) int {
		return int(l.Seq()) - int(r.Seq())
	})
	return leaders
}

// GetRound collects the highest observable sequence number of the autocrat's
// events in the past cone of the provided event.
func (a *Autocracy) GetRound(event *model.Event) uint32 {
	if round, exists := a.roundsCache[event.EventId()]; exists {
		return round
	}

	if event.Creator() == a.autocrat {
		round := event.Seq()
		a.roundsCache[event.EventId()] = round
		return round
	}

	round := uint32(0)
	for _, parent := range event.Parents() {
		round = max(round, a.GetRound(parent))
	}
	a.roundsCache[event.EventId()] = round
	return round
}
