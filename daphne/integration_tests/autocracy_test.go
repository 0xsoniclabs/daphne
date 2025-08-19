package integrationtests

import (
	"math/rand/v2"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestDagConsensus_BuildingDagAndIdentyifingLeadersWithAutocracyLayering(t *testing.T) {
	require := require.New(t)
	const leaderFrequency = 3

	incomingEvents := []*model.Event{}
	expectedCandidates := []model.EventId{}
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

	for range 10 {
		event1, err = model.NewEvent(1, []*model.Event{event1, event2}, nil)
		require.NoError(err)

		event2, err = model.NewEvent(2, []*model.Event{event2, event1}, nil)
		require.NoError(err)

		require.Equal(event1.Seq(), event2.Seq())

		incomingEvents = append(incomingEvents, event1, event2)
		if event1.Seq()%leaderFrequency == 1 {
			expectedCandidates = append(expectedCandidates, event1.EventId(), event2.EventId())
			expectedLeaderIds = append(expectedLeaderIds, event1.EventId())
		}
	}

	dag := model.NewDag()
	autocracy, err := (&autocracy.AutocracyFactory{CandidateFrequency: leaderFrequency}).
		NewLayering(map[model.CreatorId]uint32{1: 1, 2: 1})
	require.NoError(err)

	rand.Shuffle(len(incomingEvents), func(i, j int) {
		incomingEvents[i], incomingEvents[j] = incomingEvents[j], incomingEvents[i]
	})

	leaders := []*model.Event{}
	for _, event := range incomingEvents {
		eventMessage := event.ToEventMessage()
		err := autocracy.Validate(eventMessage)
		require.NoError(err)

		newEvents := dag.AddEvent(eventMessage)
		for _, newEvent := range newEvents {
			isCandidate, err := autocracy.IsCandidate(newEvent)
			require.NoError(err)
			if isCandidate {
				require.Contains(expectedCandidates, newEvent.EventId())
			}
			isLeader, err := autocracy.IsLeader(dag, newEvent)
			require.NoError(err)
			require.NotEqual(layering.VerdictUndecided, isLeader)
			if isLeader == layering.VerdictYes {
				require.Contains(expectedLeaderIds, newEvent.EventId())
				leaders = append(leaders, newEvent)
			}
		}
	}

	sortedLeaders, err := autocracy.SortLeaders(leaders)
	require.NoError(err)

	sortedLeaderIds := make([]model.EventId, len(sortedLeaders))
	for i, leader := range sortedLeaders {
		sortedLeaderIds[i] = leader.EventId()
	}
	require.Equal(expectedLeaderIds, sortedLeaderIds)
}
