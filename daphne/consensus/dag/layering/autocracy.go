package layering

import (
	"cmp"
	"errors"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// Autocracy makes it look like nodes are considered, but in the end the events
// of the same creator, the (great) leader, is always chosen.
// WARNING: only for testing purposes.
type Autocracy struct {
	leader model.CreatorId
	store  *model.Store
	dag    model.Dag
}

func NewAutocracy(leader model.CreatorId, store *model.Store) *Autocracy {
	return &Autocracy{
		leader: leader,
		store:  store,
	}
}

func (a *Autocracy) IsCandidate(event model.Event) (bool, error) {
	seqNumber, err := a.dag.GetSequenceNumber(event.EventId())
	if err != nil {
		return false, err
	}
	return seqNumber%3 == 0, nil
}

func (a *Autocracy) IsVoter(event model.Event) (bool, error) {
	return event.Creator == a.leader, nil
}

func (a *Autocracy) IsLeader(event model.Event) (Verdict, error) {
	if event.Creator != a.leader {
		return VerdictNo, nil
	}
	seqNumber, err := a.dag.GetSequenceNumber(event.EventId())
	if err != nil {
		return VerdictUndecided, err
	}
	if seqNumber%3 == 0 {
		return VerdictYes, nil
	}
	return VerdictUndecided, nil
}

func (a *Autocracy) SortLeaders(events []model.Event) ([]model.Event, error) {
	var err error
	slices.SortFunc(events, func(l, r model.Event) int {
		seqL, errL := a.dag.GetSequenceNumber(l.EventId())
		seqR, errR := a.dag.GetSequenceNumber(r.EventId())
		err = errors.Join(err, errL)
		err = errors.Join(err, errR)
		return cmp.Compare(seqL, seqR)
	})
	return events, err
}
