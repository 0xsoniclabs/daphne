package autocracy

import (
	"errors"
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type AutocracyFactory struct {
	candidateFrequency uint32
}

// Autocracy makes it look like nodes are considered, but in the end the events
// of the same creator, the (great) leader, is always chosen.
// WARNING: only for testing purposes.
type Autocracy struct {
	dag                *model.Dag
	committee          map[model.CreatorId]uint32
	leader             model.CreatorId
	candidateFrequency uint32
}

func (af AutocracyFactory) NewLayering(
	dag *model.Dag,
	committee map[model.CreatorId]uint32,
) (layering.Layering, error) {
	return newAutocracy(dag, committee, af.candidateFrequency)
}

func newAutocracy(
	dag *model.Dag,
	committee map[model.CreatorId]uint32,
	candidateFrequency uint32,
) (*Autocracy, error) {
	if dag == nil {
		return nil, errors.New("nil DAG provided")
	}
	if len(committee) == 0 {
		return nil, errors.New("empty committee provided")
	}
	return &Autocracy{
		dag:                dag,
		leader:             slices.Min(slices.Collect(maps.Keys(committee))),
		candidateFrequency: candidateFrequency,
	}, nil
}

func (a *Autocracy) CheckCompatibility(event model.EventMessage) error {
	if len(event.Parents) < 2 {
		return errors.New("autocracy requires at least two parents")
	}
	if _, ok := a.committee[event.Creator]; !ok {
		return errors.New("event creator is not in the committee")
	}
	return nil
}

// IsCandidate returns true for periodic events.
func (a *Autocracy) IsCandidate(event *model.Event) (bool, error) {
	return event.Seq()%a.candidateFrequency == 0, nil
}

func (a *Autocracy) IsLeader(event *model.Event) (layering.Verdict, error) {
	isCandidate, _ := a.IsCandidate(event)
	if !isCandidate {
		return layering.VerdictNo, nil
	}
	if event.Creator() == a.leader {
		return layering.VerdictYes, nil
	}
	return layering.VerdictUndecided, nil
}

func (a *Autocracy) SortLeaders(events []*model.Event) ([]*model.Event, error) {
	slices.SortFunc(events, func(l, r *model.Event) int {
		return int(l.Seq() - r.Seq())
	})
	return events, nil
}
