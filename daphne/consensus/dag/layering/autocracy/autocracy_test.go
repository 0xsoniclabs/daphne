package autocracy

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestAutocracy_NewLayering_ErrorOnEmptyCommittee(t *testing.T) {
	_, err := (&AutocracyFactory{}).NewLayering(map[model.CreatorId]uint32{})
	require.ErrorContains(t, err, "empty committee")

	_, err = (&AutocracyFactory{}).NewLayering(nil)
	require.ErrorContains(t, err, "empty committee")
}

func TestAutocracy(t *testing.T) {
	require := require.New(t)
	committee := map[model.CreatorId]uint32{
		1: 1,
		2: 1,
	}

	autocracy, err := newAutocracy(committee, 3)
	require.NoError(err)

	require.False(autocracy.IsCandidate(nil, ChainEvent(3)))
	require.True(autocracy.IsCandidate(nil, ChainEvent(4)))
}

// func TestAutocracy_NewAutocracy_CorrectlyInitializes(t *testing.T) {
// 	require := require.New(t)
// 	candidateFrequency := uint32(3)

// 	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, candidateFrequency)
// 	require.NoError(err)
// 	require.NotNil(autocracy)
// 	require.Equal(model.CreatorId(1), autocracy.leader)
// 	require.Equal(candidateFrequency, autocracy.candidateFrequency)
// }

// func TestAutocracy_IsCandidate_ReturnsTruePeriodically(t *testing.T) {
// 	autocracy := &Autocracy{leader: model.CreatorId(0), candidateFrequency: 3}

// 	tests := map[*model.Event]bool{}
// 	for i := range 10 {
// 		event, err := model.NewEvent(uint32(i), model.CreatorId(i), nil, nil)
// 		require.NoError(t, err)
// 		tests[event] = i%int(autocracy.candidateFrequency) == 0
// 	}

// 	for event, expected := range tests {
// 		t.Run(fmt.Sprintf("%+v", *event), func(t *testing.T) {
// 			isCandidate, err := autocracy.IsCandidate(nil, event)
// 			require.NoError(t, err)
// 			require.Equal(t, expected, isCandidate)
// 		})
// 	}
// }

// func TestAutocracy_IsLeader_ReturnsTrueForAutocrat(t *testing.T) {
// 	require := require.New(t)

// 	autocracy := &Autocracy{leader: model.CreatorId(0), candidateFrequency: 3}

// 	tests := map[*model.Event]layering.Verdict{}

// 	event, err := model.NewEvent(uint32(1), model.CreatorId(1), nil, nil)
// 	require.NoError(err)
// 	tests[event] = layering.VerdictNo

// 	event, err = model.NewEvent(uint32(0), model.CreatorId(1), nil, nil)
// 	require.NoError(err)
// 	tests[event] = layering.VerdictUndecided

// 	event, err = model.NewEvent(uint32(0), model.CreatorId(0), nil, nil)
// 	require.NoError(err)
// 	tests[event] = layering.VerdictYes

// 	for event, expected := range tests {
// 		t.Run(fmt.Sprintf("%+v", *event), func(t *testing.T) {
// 			isLeader, err := autocracy.IsLeader(nil, event)
// 			require.NoError(err)
// 			require.Equal(expected, isLeader)
// 		})
// 	}
// }

// func TestAutocracy_SortLeaders_ReturnsEventsSortedBySeq(t *testing.T) {
// 	require := require.New(t)

// 	autocracy := &Autocracy{leader: model.CreatorId(0), candidateFrequency: 3}

// 	leaders := []*model.Event{}
// 	for i := range 100 {
// 		event, err := model.NewEvent(uint32(i), 0, nil, nil)
// 		require.NoError(err)
// 		leaders = append(leaders, event)
// 	}

// 	shuffledLeaders := slices.Clone(leaders)
// 	rand.Shuffle(len(shuffledLeaders), func(i, j int) {
// 		shuffledLeaders[i], shuffledLeaders[j] = shuffledLeaders[j], shuffledLeaders[i]
// 	})

// 	sortedLeaders, err := autocracy.SortLeaders(shuffledLeaders)
// 	require.NoError(err)
// 	require.ElementsMatch(leaders, sortedLeaders)
// }

func ChainEvent(num int) *model.Event {
	var selfParent, event *model.Event = nil, nil
	var err error
	for range num {
		var parents []*model.Event
		var seq uint32
		if selfParent == nil {
			parents = nil
			seq = 0
		} else {
			parents = []*model.Event{selfParent}
			seq = selfParent.Seq() + 1
		}
		event, err = model.NewEvent(seq, 0, parents, nil)
		if err != nil {
			panic(err)
		}
		selfParent = event
	}
	return event
}
