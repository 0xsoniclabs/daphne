package autocracy

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestAutocracy_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Autocracy{}
}

func TestAutocracy_NewAutocracy_SetsDefaultCandidateFrequencyWhenZeroIsProvided(t *testing.T) {
	autocracy := newAutocracy(newSimpleCommittee(t, 1), 0)
	require.Equal(t, DefaultCandidateFrequency, autocracy.candidateFrequency)
}

func TestAutocracy_NewAutocracy_CorrectlyInitializesFields(t *testing.T) {
	require := require.New(t)
	candidateFrequency := uint32(3)

	autocracy := newAutocracy(newSimpleCommittee(t, 2), candidateFrequency)
	require.NotNil(autocracy)
	// Autocrat is a creator with the lowest ID
	require.Equal(model.CreatorId(1), autocracy.autocrat)
	require.Equal(candidateFrequency, autocracy.candidateFrequency)
}

func TestAutocracy_IsCandidate(t *testing.T) {
	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(newSimpleCommittee(t, 2))

	tests := map[string]struct {
		creator                 model.CreatorId
		chainLength             int
		expectedCandidateStatus bool
	}{
		"nil event": {
			chainLength:             0,
			expectedCandidateStatus: false,
		},
		"creator not in committee": {
			chainLength:             1,
			creator:                 3,
			expectedCandidateStatus: false,
		},
		"not periodic": {
			chainLength:             2,
			creator:                 1,
			expectedCandidateStatus: false,
		},
		"periodic": {
			chainLength:             4,
			creator:                 1,
			expectedCandidateStatus: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event, _ := selfParentEventChain(t, tc.creator, tc.chainLength)
			verdict := autocracy.IsCandidate(event)
			require.Equal(t, tc.expectedCandidateStatus, verdict)
		})
	}
}

func TestAutocracy_IsLeader_ReturnsVerdictNoForTrivialConditions(t *testing.T) {
	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(newSimpleCommittee(t, 2))

	tests := map[string]struct {
		creator     model.CreatorId
		chainLength int
	}{
		"nil event": {
			chainLength: 0,
		},
		"creator not in committee": {
			chainLength: 1,
			creator:     3,
		},
		"not periodic": {
			chainLength: 2,
			creator:     1,
		},
		"periodic but not from autocrat": {
			chainLength: 4,
			creator:     2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event, dag := selfParentEventChain(t, tc.creator, tc.chainLength)
			verdict := autocracy.IsLeader(dag, event)
			require.Equal(t, layering.VerdictNo, verdict)
		})
	}
}

func TestAutocracy_IsLeader_ReturnsVerdictUndecided(t *testing.T) {
	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(newSimpleCommittee(t, 2))

	event, dag := selfParentEventChain(t, 1, 1)
	// not seen by any autocrat event.
	verdict := autocracy.IsLeader(dag, event)
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, dag = selfParentEventChain(t, 1, 2)
	// seen by a single autoract event, but not by a candidate autocrat event
	verdict = autocracy.IsLeader(dag, event.SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, dag = selfParentEventChain(t, 1, 3)
	// seen by a two autoract even, but not by a candidate autocrat eventt
	verdict = autocracy.IsLeader(dag, event.SelfParent().SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, _ = selfParentEventChain(t, 1, 4)
	// seen by another candidate, but inconsistent DAG passed
	verdict = autocracy.IsLeader(model.NewDag(), event.SelfParent().SelfParent().SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestAutocracy_IsLeader_ReturnsVerdictYes(t *testing.T) {
	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(newSimpleCommittee(t, 2))

	event, dag := selfParentEventChain(t, 1, 4)
	// Take the autocrat candidate that's seen by a younger autocrat candidate
	event = event.SelfParent().SelfParent().SelfParent()
	verdict := autocracy.IsLeader(dag, event)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestAutocracy_SortLeaders_ReturnsLeadersSortedBySeq(t *testing.T) {
	require := require.New(t)

	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(newSimpleCommittee(t, 2))

	eventIterator, dag := selfParentEventChain(t, 1, 100)
	events := []*model.Event{}
	expectedLeaders := []*model.Event{}
	for eventIterator != nil {
		if autocracy.IsLeader(dag, eventIterator) == layering.VerdictYes {
			expectedLeaders = append(expectedLeaders, eventIterator)
		}
		events = append(events, eventIterator)
		eventIterator = eventIterator.SelfParent()
	}
	slices.Reverse(expectedLeaders)

	rand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	sortedLeaders := autocracy.SortLeaders(dag, events)
	require.Equal(expectedLeaders, sortedLeaders)
}

// newSimpleCommittee is a helper method that creates a committee with the
// specified size and uniform stake.
func newSimpleCommittee(t *testing.T, size int) *consensus.Committee {
	t.Helper()
	committeeMap := map[model.CreatorId]uint32{}
	for i := 1; i <= size; i++ {
		committeeMap[model.CreatorId(i)] = 1
	}
	committee, err := consensus.NewCommittee(committeeMap)
	require.NoError(t, err)
	return committee
}

// selfParentEventChain is a helper method that creates a single creator event chain
// starting from the startingEvent. The methods creates chainLength number of new events
// and a Dag instance populated with created events.
func selfParentEventChain(
	t *testing.T,
	creator model.CreatorId,
	chainLength int,
) (*model.Event, *model.Dag) {
	t.Helper()

	dag := model.NewDag()
	if chainLength == 0 {
		return nil, dag
	}
	var addedEvents []*model.Event
	for range chainLength {
		var parents []model.EventId
		if len(addedEvents) == 0 {
			parents = nil
		} else {
			parents = []model.EventId{addedEvents[0].EventId()}
		}
		eventMessage := model.EventMessage{
			Creator: creator,
			Parents: parents,
		}
		addedEvents = dag.AddEvent(eventMessage)
		require.Len(t, addedEvents, 1)
		require.Equal(t, addedEvents[0].Creator(), creator)
	}
	return addedEvents[0], dag
}
