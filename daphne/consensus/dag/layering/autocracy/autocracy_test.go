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

var _ layering.Factory = Factory{}

func TestFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := Factory{}
	require.Equal(t, "autocracy-freq=3", factory.String())

	factory = Factory{CandidateFrequency: 5}
	require.Equal(t, "autocracy-freq=5", factory.String())
}

func TestAutocracy_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Autocracy{}
}

func TestAutocracy_NewAutocracy_SetsDefaultCandidateFrequencyWhenZeroIsProvided(t *testing.T) {
	committee := consensus.NewUniformCommittee(1)
	autocracy := newAutocracy(model.NewDag(committee), committee, 0)
	require.Equal(t, DefaultCandidateFrequency, autocracy.candidateFrequency)
}

func TestAutocracy_NewAutocracy_CorrectlyInitializesFields(t *testing.T) {
	require := require.New(t)
	candidateFrequency := uint32(3)
	committee := consensus.NewUniformCommittee(2)
	autocracy := newAutocracy(model.NewDag(committee), committee, candidateFrequency)
	require.NotNil(autocracy)
	// Autocrat is a creator with the lowest ID
	require.Equal(consensus.ValidatorId(0), autocracy.autocrat)
	require.Equal(candidateFrequency, autocracy.candidateFrequency)
}

func TestAutocracy_IsCandidate(t *testing.T) {
	committee := consensus.NewUniformCommittee(2)
	autocracy := (&Factory{CandidateFrequency: 3}).NewLayering(model.NewDag(committee), committee)

	tests := map[string]struct {
		creator                 consensus.ValidatorId
		chainLength             int
		expectedCandidateStatus bool
	}{
		"nil event": {
			chainLength:             0,
			expectedCandidateStatus: false,
		},
		"not periodic": {
			chainLength:             2,
			creator:                 0,
			expectedCandidateStatus: false,
		},
		"periodic": {
			chainLength:             4,
			creator:                 0,
			expectedCandidateStatus: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event, _ := selfParentEventChain(t, tc.creator, committee, tc.chainLength)
			verdict := autocracy.IsCandidate(event)
			require.Equal(t, tc.expectedCandidateStatus, verdict)
		})
	}
}

func TestAutocracy_IsLeader_ReturnsVerdictNoForTrivialConditions(t *testing.T) {
	tests := map[string]struct {
		creator     consensus.ValidatorId
		chainLength int
	}{
		"nil event": {
			chainLength: 0,
		},
		"not periodic": {
			chainLength: 2,
			creator:     0,
		},
		"periodic but not from autocrat": {
			chainLength: 4,
			creator:     1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			committee := consensus.NewUniformCommittee(2)
			event, dag := selfParentEventChain(t, tc.creator, committee, tc.chainLength)
			autocracy := newAutocracy(dag, committee, 3)
			verdict := autocracy.IsLeader(event)
			require.Equal(t, layering.VerdictNo, verdict)
		})
	}
}

func TestAutocracy_IsLeader_ReturnsVerdictUndecided(t *testing.T) {
	committee := consensus.NewUniformCommittee(2)

	event, dag := selfParentEventChain(t, 0, committee, 1)
	autocracy := newAutocracy(dag, committee, 3)
	// not seen by any autocrat event.
	verdict := autocracy.IsLeader(event)
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, dag = selfParentEventChain(t, 0, committee, 2)
	autocracy = newAutocracy(dag, committee, 3)
	// seen by a single autoract event, but not by a candidate autocrat event
	verdict = autocracy.IsLeader(event.SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, dag = selfParentEventChain(t, 0, committee, 3)
	autocracy = newAutocracy(dag, committee, 3)
	// seen by a two autoract even, but not by a candidate autocrat eventt
	verdict = autocracy.IsLeader(event.SelfParent().SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)

	event, _ = selfParentEventChain(t, 0, committee, 4)
	autocracy = newAutocracy(model.NewDag(committee), committee, 3)
	// seen by another candidate, with inconsistent DAG
	verdict = autocracy.IsLeader(event.SelfParent().SelfParent().SelfParent())
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestAutocracy_IsLeader_ReturnsVerdictYes(t *testing.T) {
	committee := consensus.NewUniformCommittee(2)
	event, dag := selfParentEventChain(t, 0, committee, 4)
	autocracy := newAutocracy(dag, committee, 3)
	// Take the autocrat candidate that's seen by a younger autocrat candidate
	event = event.SelfParent().SelfParent().SelfParent()
	verdict := autocracy.IsLeader(event)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestAutocracy_SortLeaders_ReturnsLeadersSortedBySeq(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(2)
	eventIterator, dag := selfParentEventChain(t, 0, committee, 100)
	autocracy := newAutocracy(dag, committee, 3)

	events := []*model.Event{}
	expectedLeaders := []*model.Event{}
	for eventIterator != nil {
		if autocracy.IsLeader(eventIterator) == layering.VerdictYes {
			expectedLeaders = append(expectedLeaders, eventIterator)
		}
		events = append(events, eventIterator)
		eventIterator = eventIterator.SelfParent()
	}
	slices.Reverse(expectedLeaders)

	rand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	sortedLeaders := autocracy.SortLeaders(events)
	require.Equal(expectedLeaders, sortedLeaders)
}

func TestAutocracy_GetRound_ReturnsSequenceNumberOfLatestAutocratEvent(t *testing.T) {
	require := require.New(t)

	autocrat := consensus.ValidatorId(12)
	other := consensus.ValidatorId(34)

	autocracy := &Autocracy{
		autocrat:    autocrat,
		roundsCache: map[model.EventId]uint32{},
	}

	// The autocrat's genesis event is considered round 1.
	eventAutocrat0, err := model.NewEvent(autocrat, nil, nil)
	require.NoError(err)
	require.EqualValues(1, autocracy.GetRound(eventAutocrat0))

	// Non-autocrat genesis events see no autocrat events, so they are round 0.
	eventOther0, err := model.NewEvent(other, nil, nil)
	require.NoError(err)
	require.EqualValues(0, autocracy.GetRound(eventOther0))

	// Subsequent autocrat events increase the round.
	eventAutocrat1, err := model.NewEvent(autocrat, []*model.Event{eventAutocrat0}, nil)
	require.NoError(err)
	require.EqualValues(2, autocracy.GetRound(eventAutocrat1))

	eventAutocrat2, err := model.NewEvent(autocrat, []*model.Event{eventAutocrat1}, nil)
	require.NoError(err)
	require.EqualValues(3, autocracy.GetRound(eventAutocrat2))

	// Non-autocrat events use the latest autocrat event they see.
	eventOther1, err := model.NewEvent(other, []*model.Event{eventOther0}, nil)
	require.NoError(err)
	require.EqualValues(0, autocracy.GetRound(eventOther1))

	eventOther2, err := model.NewEvent(other, []*model.Event{eventOther1, eventAutocrat1}, nil)
	require.NoError(err)
	require.EqualValues(2, autocracy.GetRound(eventOther2))
}

// selfParentEventChain is a helper method that creates a single creator event chain
// starting from the startingEvent. The methods creates chainLength number of new events
// and a Dag instance populated with created events.
func selfParentEventChain(
	t *testing.T,
	creator consensus.ValidatorId,
	committee *consensus.Committee,
	chainLength int,
) (*model.Event, model.Dag) {
	t.Helper()

	dag := model.NewDag(committee)
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
