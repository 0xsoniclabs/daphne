package autocracy

import (
	"fmt"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type AutocracyFactory struct {
	Autocrat model.CreatorId
}

// Autocracy makes it look like nodes are considered, but in the end the events
// of the same creator, the (great) leader, is always chosen.
// WARNING: only for testing purposes.
type Autocracy struct {
	leader model.CreatorId
}

func (af AutocracyFactory) NewLayering(dag *model.Dag, committee map[model.CreatorId]uint32) layering.Layering {
	return &Autocracy{
		leader: af.Autocrat,
	}
}

func (a *Autocracy) IsCandidate(event *model.Event) (bool, error) {
	return event.Seq%3 == 0, nil
}

func (a *Autocracy) IsLeader(event *model.Event) (layering.Verdict, error) {
	fmt.Println(event.Creator)
	fmt.Println(a.leader)
	if event.Creator != a.leader {
		return layering.VerdictNo, nil
	}
	if event.Seq%3 == 0 {
		return layering.VerdictYes, nil
	}
	return layering.VerdictUndecided, nil
}

func (a *Autocracy) SortLeaders(events []*model.Event) ([]*model.Event, error) {
	slices.SortFunc(events, func(l, r *model.Event) int {
		return int(l.Seq - r.Seq)
	})
	return events, nil
}
