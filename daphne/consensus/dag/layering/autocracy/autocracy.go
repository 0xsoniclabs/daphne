package autocracy

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// Autocracy is a simple testing layering that makes it look like all creators are considered,
// but in the end the events of the same creator, the autocrat, are always chosen.
// Autocrat is defined as the creator with the lowest ID in the associated committee.
// The Autocracy layers the DAG by identifying candidates as periodic events.
// The period is configurable.
// A candidate is every candidateFrequency-th valid event created by the same creator with a
// valid recursive history up to its genesis event.
// A leader is every candidate event created by the committee autocrat.
type Autocracy struct {
	committee          map[model.CreatorId]uint32
	autocrat           model.CreatorId
	candidateFrequency uint32

	candidateCache map[model.EventId]bool
}

func (af AutocracyFactory) NewLayering(
	committee map[model.CreatorId]uint32,
) (layering.Layering, error) {
	return newAutocracy(committee, af.CandidateFrequency)
}

type AutocracyFactory struct {
	CandidateFrequency uint32
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
		autocrat:           slices.Min(slices.Collect(maps.Keys(committee))),
		candidateFrequency: candidateFrequency,
		candidateCache:     make(map[model.EventId]bool),
	}, nil
}

// [Autocracy.Validate] checks if the provided event message is not nil and
// if its creator is within the associated committee.
func (a *Autocracy) Validate(eventMessage model.EventMessage) error {
	if _, exists := a.committee[eventMessage.Creator]; !exists {
		return errors.New("event creator is not in committee")
	}
	return nil
}

// validate is an internal validation method that is a superset of the
// public [Autocracy.Validate] method. It is to be used while traversing
// the DAG.
func (a *Autocracy) validate(event *model.Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	return a.Validate(event.ToEventMessage())
}

// IsCandidate returns true for periodic events. For an event to be valid
// its self-parent chain down to its creator's genesis event has to be valid
func (a *Autocracy) IsCandidate(event *model.Event) (bool, error) {
	if err := a.validate(event); err != nil {
		return false, err
	}
	// If the genesis is reached without an error or chain breakage, the event is a candidate.
	if event.IsGenesis() {
		return true, nil
	}
	if isCandidate, exists := a.candidateCache[event.EventId()]; exists {
		return isCandidate, nil
	}
	// Iterate down the self-parent chain by candidateFrequency steps.
	eventIterator := event
	for range a.candidateFrequency {
		eventIterator = eventIterator.SelfParent()
		// If the next self-parent is nil, the chain length is not 1 (mod candidateFrequency)
		if eventIterator == nil {
			a.candidateCache[event.EventId()] = false
			return false, nil
		}
		if err := a.validate(eventIterator); err != nil {
			return false, err
		}
	}
	isCandidate, err := a.IsCandidate(eventIterator)
	// Cache only valid event results
	if err == nil {
		a.candidateCache[event.EventId()] = isCandidate
	}
	return isCandidate, err
}

// [Autocracy.IsLeader] declares every autocrat's candidate event as a leader.
// All other events are reported as not being leaders. Autocracy has a simple, non voting-based
// leader election, meaning that the provided dag is ignored, as well as the
// [layering.VerdictUndecided] is never being returned.
// If the event is invalid, an error is returned.
func (a *Autocracy) IsLeader(dag *model.Dag, event *model.Event) (layering.Verdict, error) {
	isCandidate, err := a.IsCandidate(event)
	if err != nil {
		return layering.VerdictNo, err
	}
	if !isCandidate {
		return layering.VerdictNo, nil
	}
	if event.Creator() == a.autocrat {
		return layering.VerdictYes, nil
	}
	return layering.VerdictNo, nil
}

// [Autocracy.SortLeaders] verifies the leader status of the passed events and given the simple
// periodicity election, returns them sorted by their sequence number.
// If any of the provided events is invalid or is not a leader, an error is returned.
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
		return int(l.Seq()) - int(r.Seq())
	})
	return events, nil
}
