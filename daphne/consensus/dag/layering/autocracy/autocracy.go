package autocracy

import (
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
	committee *consensus.Committee,
) layering.Layering {
	return newAutocracy(committee, af.CandidateFrequency)
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
	committee          *consensus.Committee
	autocrat           consensus.ValidatorId
	candidateFrequency uint32
}

func newAutocracy(
	committee *consensus.Committee,
	candidateFrequency uint32,
) *Autocracy {
	if candidateFrequency == 0 {
		candidateFrequency = DefaultCandidateFrequency
	}
	return &Autocracy{
		committee:          committee,
		autocrat:           slices.Min(committee.Validators()),
		candidateFrequency: candidateFrequency,
	}
}

// IsCandidate returns true for periodic events created by any committee member.
func (a *Autocracy) IsCandidate(dag *model.Dag, event *model.Event) bool {
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
func (a *Autocracy) IsLeader(dag *model.Dag, event *model.Event) layering.Verdict {
	if !a.IsCandidate(dag, event) || a.autocrat != event.Creator() {
		return layering.VerdictNo
	}

	// Check if there is a successor autocrat candidate that reaches this event.
	heads := dag.GetHeads()
	youngestAutocrat, exists := heads[a.autocrat]
	if !exists {
		return layering.VerdictUndecided
	}
	// Find the youngest autocrat candidate.
	for !a.IsCandidate(dag, youngestAutocrat) {
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
func (a *Autocracy) SortLeaders(dag *model.Dag, events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return a.IsLeader(dag, event) != layering.VerdictYes
	})
	slices.SortFunc(leaders, func(l, r *model.Event) int {
		return int(l.Seq()) - int(r.Seq())
	})
	return leaders
}
