package autocracy

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestDagConsensus_Autocracy_BuildDagAndIdentifyLeaders(t *testing.T) {
	require := require.New(t)
	const (
		leaderFrequency     = 3
		eventSequenceLength = 10
	)

	incomingEvents := []*model.Event{}
	expectedCandidates := []model.EventId{}
	// expectedUndecidedIds are all events that are going to be undecided at some point.
	expectedUndecidedIds := []model.EventId{}
	expectedLeaderIds := []model.EventId{}

	event1, err := model.NewEvent(1, nil, nil)
	require.NoError(err)
	incomingEvents = append(incomingEvents, event1)
	// Genesis is always a candidate.
	expectedCandidates = append(expectedCandidates, event1.EventId())
	// Creator 1 is the autocrat, event 1 is going to be a leader,
	expectedLeaderIds = append(expectedLeaderIds, event1.EventId())
	// And an udecided event at one point in time.
	expectedUndecidedIds = append(expectedUndecidedIds, event1.EventId())

	// Creator two is going to produce candidates too, but never leaders.
	event2, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	incomingEvents = append(incomingEvents, event2)
	expectedCandidates = append(expectedCandidates, event2.EventId())

	for i := event1.Seq(); i < eventSequenceLength; i++ {
		event1, err = model.NewEvent(1, []*model.Event{event1, event2}, nil)
		require.NoError(err)

		event2, err = model.NewEvent(2, []*model.Event{event2, event1}, nil)
		require.NoError(err)

		require.Equal(event1.Seq(), event2.Seq())

		incomingEvents = append(incomingEvents, event1, event2)
		if event1.Seq()%leaderFrequency == 1 {
			expectedCandidates = append(expectedCandidates, event1.EventId(), event2.EventId())
			expectedUndecidedIds = append(expectedUndecidedIds, event1.EventId())
			// All autocrat candidates except the last one (seq in range [1, HighestSeq-ElectionFrequency])
			// are going to be leaders.
			if event1.Seq() <= eventSequenceLength-leaderFrequency {
				expectedLeaderIds = append(expectedLeaderIds, event1.EventId())
			}
		}
	}

	committee := newSimpleCommittee(t, 2)
	dag := model.NewDag(committee)
	autocracy := (&Factory{CandidateFrequency: leaderFrequency}).NewLayering(committee)

	rand.Shuffle(len(incomingEvents), func(i, j int) {
		incomingEvents[i], incomingEvents[j] = incomingEvents[j], incomingEvents[i]
	})

	currentCandidate := []*model.Event{}
	leaders := []*model.Event{}
	for _, event := range incomingEvents {
		eventMessage := event.ToEventMessage()
		newEvents := dag.AddEvent(eventMessage)
		for _, newEvent := range newEvents {
			if autocracy.IsCandidate(newEvent) {
				require.Contains(expectedCandidates, newEvent.EventId())

				currentCandidate = append(currentCandidate, newEvent)
			}
			newCandidates := slices.Clone(currentCandidate)
			for _, candidate := range currentCandidate {
				leaderStatus := autocracy.IsLeader(dag, candidate)
				switch leaderStatus {
				case layering.VerdictNo:
					newCandidates = slices.DeleteFunc(newCandidates, func(e *model.Event) bool {
						return e.EventId() == candidate.EventId()
					})
				case layering.VerdictUndecided:
					require.Contains(expectedUndecidedIds, candidate.EventId())
				case layering.VerdictYes:
					require.Contains(expectedLeaderIds, candidate.EventId())

					leaders = append(leaders, candidate)
					newCandidates = slices.DeleteFunc(newCandidates, func(e *model.Event) bool {
						return e.EventId() == candidate.EventId()
					})
				}
			}
			currentCandidate = newCandidates
		}
	}

	sortedLeaders := autocracy.SortLeaders(dag, leaders)

	sortedLeaderIds := make([]model.EventId, len(sortedLeaders))
	for i, leader := range sortedLeaders {
		sortedLeaderIds[i] = leader.EventId()
	}
	require.Equal(expectedLeaderIds, sortedLeaderIds)
}
