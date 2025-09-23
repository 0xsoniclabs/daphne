package lachesis

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestLachesis_IsCandidate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{1: 1, 2: 1})
	require.NoError(t, err)

	lachesis := (&Factory{}).NewLayering(committee).(*Lachesis)
	dag := model.NewDag()

	// ╬════════e_2_5
	// ║          ║
	// e_1_5══════╬
	// ║          ║
	// ╬════════e_2_4
	// ║          ║
	// ╬════════e_2_3
	// ║          ║
	// e_1_4══════╬
	// ║          ║
	// e_1_3══════╬
	// ║          ║
	// ╬════════e_2_2
	// ║          ║
	// e_1_2══════╬
	// ║          ║
	// e_1_1    e_2_1

	// e_#creatorid_#seq
	e_1_1 := newEvent(t, 1, nil)
	e_2_1 := newEvent(t, 2, nil)

	e_1_2 := newEvent(t, 1, []*model.Event{e_1_1, e_2_1})

	require.False(t, lachesis.stronglyReaches(e_1_2, e_1_1))
	require.True(t, lachesis.stronglyReaches(e_1_2, e_2_1))

	require.True(t, lachesis.IsCandidate(e_1_1))
	require.True(t, lachesis.IsCandidate(e_2_1))
	require.False(t, lachesis.IsCandidate(e_1_2))

	e_2_2, _ := model.NewEvent(2, []*model.Event{e_2_1, e_1_2}, nil)
	e_1_3, _ := model.NewEvent(1, []*model.Event{e_1_2, e_2_2}, nil)

	require.True(t, lachesis.IsCandidate(e_2_2))
	require.True(t, lachesis.IsCandidate(e_1_3))

	e_1_4, _ := model.NewEvent(1, []*model.Event{e_1_3, e_2_2}, nil)
	require.False(t, lachesis.IsCandidate(e_1_4))

	e_2_3, _ := model.NewEvent(2, []*model.Event{e_2_2, e_1_4}, nil)
	require.True(t, lachesis.IsCandidate(e_2_3))

	e_1_1 = dag.AddEvent(e_1_1.ToEventMessage())[0]
	e_2_1 = dag.AddEvent(e_2_1.ToEventMessage())[0]
	e_1_2 = dag.AddEvent(e_1_2.ToEventMessage())[0]
	e_2_2 = dag.AddEvent(e_2_2.ToEventMessage())[0]
	e_1_3 = dag.AddEvent(e_1_3.ToEventMessage())[0]
	e_1_4 = dag.AddEvent(e_1_4.ToEventMessage())[0]
	e_2_3 = dag.AddEvent(e_2_3.ToEventMessage())[0]

	require.Equal(t, 3, lachesis.eventFrame(e_2_3))
	require.True(t, lachesis.IsCandidate(e_2_3))
	require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_2_3))
	require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, e_1_4))

	require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, e_2_1))
	require.Equal(t, layering.VerdictYes, lachesis.IsLeader(dag, e_1_1))
}

func TestLachesis_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Lachesis{}
}

func TestAutocracy_IsCandidate(t *testing.T) {
	autocracy := (&Factory{}).NewLayering(newSimpleCommittee(t, 2))

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
		// "periodic": {
		// 	chainLength:             4,
		// 	creator:                 1,
		// 	expectedCandidateStatus: true,
		// },
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
	autocracy := (&Factory{}).NewLayering(newSimpleCommittee(t, 2))

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
	autocracy := (&Factory{}).NewLayering(newSimpleCommittee(t, 2))

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
	autocracy := (&Factory{}).NewLayering(newSimpleCommittee(t, 2))

	event, dag := selfParentEventChain(t, 1, 4)
	// Take the autocrat candidate that's seen by a younger autocrat candidate
	event = event.SelfParent().SelfParent().SelfParent()
	verdict := autocracy.IsLeader(dag, event)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestLachesis_SortLeaders_ReturnsLeadersSortedByFrame(t *testing.T) {
	require := require.New(t)

	lachesis := (&Factory{}).NewLayering(newSimpleCommittee(t, 2))

	eventIterator, dag := selfParentEventChain(t, 1, 100)
	events := []*model.Event{}
	expectedLeaders := []*model.Event{}
	for eventIterator != nil {
		if lachesis.IsLeader(dag, eventIterator) == layering.VerdictYes {
			expectedLeaders = append(expectedLeaders, eventIterator)
		}
		events = append(events, eventIterator)
		eventIterator = eventIterator.SelfParent()
	}
	slices.Reverse(expectedLeaders)

	rand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	sortedLeaders := lachesis.SortLeaders(dag, events)
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

func newEvent(t *testing.T, creator model.CreatorId, parents []*model.Event) *model.Event {
	t.Helper()
	event, err := model.NewEvent(creator, parents, nil)
	require.NoError(t, err)
	return event
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
