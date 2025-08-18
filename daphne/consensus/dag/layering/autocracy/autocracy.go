package autocracy

import (
	"errors"
	"fmt"
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
	committee          map[model.CreatorId]uint32
	leader             model.CreatorId
	candidateFrequency uint32
}

func (af AutocracyFactory) NewLayering(
	committee map[model.CreatorId]uint32,
) (layering.Layering, error) {
	return newAutocracy(committee, af.candidateFrequency)
}

func newAutocracy(
	committee map[model.CreatorId]uint32,
	candidateFrequency uint32,
) (*Autocracy, error) {
	if len(committee) == 0 {
		return nil, errors.New("empty committee provided")
	}
	return &Autocracy{
		committee:          committee,
		leader:             slices.Min(slices.Collect(maps.Keys(committee))),
		candidateFrequency: candidateFrequency,
	}, nil
}

func (a *Autocracy) Validate(event *model.Event) error {
	// parents := event.Parents()
	// slices.DeleteFunc(parents, func(p *model.Event) bool {
	// 	return p == nil
	// })

	// if len(parents) < 2 {
	// 	return errors.New("event must have at least two parents")
	// }
	return nil
}

// IsCandidate returns true for periodic events.
func (a *Autocracy) IsCandidate(event *model.Event) (bool, error) {
	if err := a.Validate(event); err != nil {
		return false, err
	}
	if event.Seq()%a.candidateFrequency != 1 {
		return false, nil
	}
	if event.IsGenesis() {
		return true, nil
	}
	for range a.candidateFrequency {
		event = event.SelfParent()
		if event == nil {
			return false, nil
		}
		if err := a.Validate(event); err != nil {
			return false, err
		}
	}
	return a.IsCandidate(event)
}

func (a *Autocracy) IsLeader(dag *model.Dag, event *model.Event) (layering.Verdict, error) {
	isCandidate, err := a.IsCandidate(event)
	if err != nil {
		return layering.VerdictNo, err
	}
	if !isCandidate {
		return layering.VerdictNo, nil
	}
	if event.Creator() == a.leader {
		return layering.VerdictYes, nil
	}
	return layering.VerdictNo, nil
}

func (a *Autocracy) SortLeaders(events []*model.Event) ([]*model.Event, error) {
	for _, event := range events {
		leaderStatus, err := a.IsLeader(nil, event)
		if err != nil {
			return nil, fmt.Errorf("invalid event %+v: %w", event, err)
		}
		if leaderStatus != layering.VerdictYes {
			return nil, fmt.Errorf("event %+v is not a leader: %w", event, err)
		}
	}
	slices.SortFunc(events, func(l, r *model.Event) int {
		return int(l.Seq() - r.Seq())
	})
	return events, nil
}
