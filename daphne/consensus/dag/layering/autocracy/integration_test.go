package autocracy

import (
	"math/rand/v2"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestDagConsensus_Autocracy_BuildDagAndIdentyLeaders(t *testing.T) {
	require := require.New(t)
	const (
		leaderFrequency = 3
		numIterations   = 10
	)

	incomingEvents := []*model.Event{}
	expectedCandidates := []model.EventId{}
	// expectedUndecidedIds are all events that are going to be undecided at some point.
	expectedUndecidedIds := []model.EventId{}
	expectedLeaderIds := []model.EventId{}

	event1, err := model.NewEvent(1, nil, nil)
	require.NoError(err)
	incomingEvents = append(incomingEvents, event1)
	// Genesis is always a candidate
	expectedCandidates = append(expectedCandidates, event1.EventId())
	// Creator 1 is the autocrat, event 1 is going to be a leader
	expectedLeaderIds = append(expectedLeaderIds, event1.EventId())

	event2, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	incomingEvents = append(incomingEvents, event2)
	expectedCandidates = append(expectedCandidates, event2.EventId())

	for range numIterations {
		event1, err = model.NewEvent(1, []*model.Event{event1, event2}, nil)
		require.NoError(err)

		event2, err = model.NewEvent(2, []*model.Event{event2, event1}, nil)
		require.NoError(err)

		require.Equal(event1.Seq(), event2.Seq())

		incomingEvents = append(incomingEvents, event1, event2)
		if event1.Seq()%leaderFrequency == 1 {
			expectedCandidates = append(expectedCandidates, event1.EventId(), event2.EventId())
			expectedUndecidedIds = append(expectedUndecidedIds, event1.EventId())
			// If the event by a creator 1 is a candidate and has the autocrat above itself,
			// it is a leader
			if event1.Seq() <= numIterations-leaderFrequency+1 {
				expectedLeaderIds = append(expectedLeaderIds, event1.EventId())
			}
		}
	}

	dag := model.NewDag()
	autocracy := (&Factory{CandidateFrequency: leaderFrequency}).
		NewLayering(newSimpleCommittee(t, 2))

	rand.Shuffle(len(incomingEvents), func(i, j int) {
		incomingEvents[i], incomingEvents[j] = incomingEvents[j], incomingEvents[i]
	})

	leaders := []*model.Event{}
	for _, event := range incomingEvents {
		eventMessage := event.ToEventMessage()
		newEvents := dag.AddEvent(eventMessage)
		for _, newEvent := range newEvents {
			if autocracy.IsCandidate(newEvent) {
				require.Contains(expectedCandidates, newEvent.EventId())
			}
			isLeader := autocracy.IsLeader(dag, newEvent)
			if isLeader == layering.VerdictUndecided {
				require.Contains(expectedUndecidedIds, newEvent.EventId())
			}
			if isLeader == layering.VerdictYes {
				require.Contains(expectedLeaderIds, newEvent.EventId())
				leaders = append(leaders, newEvent)
			}
		}
	}

	sortedLeaders := autocracy.SortLeaders(dag, leaders)

	sortedLeaderIds := make([]model.EventId, len(sortedLeaders))
	for i, leader := range sortedLeaders {
		sortedLeaderIds[i] = leader.EventId()
	}
	require.Equal(expectedLeaderIds, sortedLeaderIds)
}
