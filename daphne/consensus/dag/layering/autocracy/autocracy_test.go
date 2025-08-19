package autocracy

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
	"unsafe"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestAutocracy_NewLayering_ErrorOnEmptyCommittee(t *testing.T) {
	_, err := (&AutocracyFactory{}).NewLayering(map[model.CreatorId]uint32{})
	require.ErrorContains(t, err, "empty committee")

	_, err = (&AutocracyFactory{}).NewLayering(nil)
	require.ErrorContains(t, err, "empty committee")
}

func TestAutocracy_NewAutocracy_CorrectlyInitializesFields(t *testing.T) {
	require := require.New(t)
	candidateFrequency := uint32(3)

	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, candidateFrequency)
	require.NoError(err)
	require.NotNil(autocracy)
	// Leader is a creator with the lowest ID
	require.Equal(model.CreatorId(1), autocracy.leader)
	require.Equal(candidateFrequency, autocracy.candidateFrequency)
}

func TestAutocracy_Validate_ReturnsErrorOnNilEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	err = autocracy.Validate(nil)
	require.ErrorContains(t, err, "event is nil")
}

func TestAutocracy_Validate_ReturnsErrorOnUnknownCreator(t *testing.T) {
	require := require.New(t)

	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(err)

	event, err := model.NewEvent(3, nil, nil)
	require.NoError(err)

	err = autocracy.Validate(event)
	require.ErrorContains(err, "creator is not in committee")
}

func TestAutocracy_IsCandidate_ReturnsErrorOnInvalidEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	// Pass events that would not pass a [Autocracy.Validate] check
	_, err = autocracy.IsCandidate(nil)
	require.Error(t, err)

	event, err := model.NewEvent(3, nil, nil)
	require.NoError(t, err)

	_, err = autocracy.IsCandidate(event)
	require.Error(t, err)
}
func TestAutocracy_IsCandidate_ReturnsErrorOnInvalidEventMidSelfParentChain(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	event := ChainEvent(t, 1, 2, nil)
	require.NotNil(t, event)

	// Creating an invalid self-parent event chain is not possible with the current Event api
	// Simulate an attacker mutating this field in a special way.
	mutatedEvent := event.Parents()[0]
	val := reflect.ValueOf(mutatedEvent).Elem()
	field := val.FieldByName("creator")
	require.True(t, field.IsValid())

	settableCreatorField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	settableCreatorField.Set(reflect.ValueOf(model.CreatorId(3)))

	_, err = autocracy.IsCandidate(event)
	require.Error(t, err)
}

func TestAutocracy_IsCandidate_ReturnsCandidateStatusBasedOnPeriodicity(t *testing.T) {
	require := require.New(t)
	const candidateFrequency = 3
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1}, candidateFrequency)
	require.NoError(err)

	tests := map[*model.Event]bool{}
	for i := range 10 {
		event := ChainEvent(t, 1, i, nil)
		if event == nil {
			continue
		}
		tests[event] = event.Seq()%candidateFrequency == 1
	}

	for event, expected := range tests {
		t.Run(fmt.Sprintf("Event Seq: %d", event.Seq()), func(t *testing.T) {
			isCandidate, err := autocracy.IsCandidate(event)
			require.NoError(err)
			require.Equal(expected, isCandidate)
		})
	}
}

func TestAutocracy_IsCandidate_CachesPreviouslyIdentifiedCandidates(t *testing.T) {
	require := require.New(t)
	const candidateFrequency = 3
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1}, candidateFrequency)
	require.NoError(err)

	event2 := ChainEvent(t, 1, 2, nil)
	require.NotNil(event2)
	event4 := ChainEvent(t, 1, 2, event2)
	require.NotNil(event4)

	isCandidate, err := autocracy.IsCandidate(event2)
	require.NoError(err)
	require.False(isCandidate)
	require.Equal(autocracy.candidateCache, map[model.EventId]bool{
		event2.EventId(): false,
	})

	isCandidate, err = autocracy.IsCandidate(event4)
	require.NoError(err)
	require.True(isCandidate)
	require.Equal(autocracy.candidateCache, map[model.EventId]bool{
		event2.EventId(): false,
		event4.EventId(): true,
	})
}

func TestAutocracy_IsLeader_ReturnsErrorForInvalidEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	// Pass events that would not pass a [Autocracy.Validate] check
	_, err = autocracy.IsLeader(nil, nil)
	require.Error(t, err)

	event, err := model.NewEvent(3, nil, nil)
	require.NoError(t, err)

	_, err = autocracy.IsLeader(nil, event)
	require.Error(t, err)
}

func TestAutocracy_IsLeader_ReturnsNoForNonCandidateEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	event := ChainEvent(t, 2, 1, nil)
	require.NotNil(t, event)

	_, err = autocracy.IsLeader(nil, event)
	require.NoError(t, err)
}

func TestAutocracy_IsLeader_ReturnsNoForNonLeaderCreatorCandidate(t *testing.T) {
	require := require.New(t)
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(err)

	// Leader is the creator with ID=1
	event := ChainEvent(t, 2, 4, nil)
	require.NotNil(t, event)

	isCandidate, err := autocracy.IsCandidate(event)
	require.NoError(err)
	require.True(isCandidate)

	isLeader, err := autocracy.IsLeader(nil, event)
	require.NoError(err)
	require.Equal(layering.VerdictNo, isLeader)
}

func TestAutocracy_IsLeader_ReturnsYesForLeaderCreatorCandidate(t *testing.T) {
	require := require.New(t)
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(err)

	// Leader is the creator with ID=1
	event := ChainEvent(t, 1, 4, nil)
	require.NotNil(t, event)

	isCandidate, err := autocracy.IsCandidate(event)
	require.NoError(err)
	require.True(isCandidate)

	isLeader, err := autocracy.IsLeader(nil, event)
	require.NoError(err)
	require.Equal(layering.VerdictYes, isLeader)
}

func TestAutocracy_SortLeaders_ReturnsErrorOnInvalidEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	// Pass events that would not pass a [Autocracy.Validate] check
	_, err = autocracy.SortLeaders([]*model.Event{nil})
	require.ErrorContains(t, err, "invalid event")

	event, err := model.NewEvent(3, nil, nil)
	require.NoError(t, err)

	_, err = autocracy.SortLeaders([]*model.Event{event})
	require.ErrorContains(t, err, "invalid event")
}

func TestAutocracy_SortLeaders_ReturnsErrorOnNonLeaderEvent(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	event := ChainEvent(t, 1, 2, nil)
	require.NotNil(t, event)

	_, err = autocracy.SortLeaders([]*model.Event{event})
	require.ErrorContains(t, err, "not a leader")
}

func TestAutocracy_SortLeaders_ReturnsLeadersSortedBySeq(t *testing.T) {
	autocracy, err := newAutocracy(map[model.CreatorId]uint32{1: 1, 2: 1}, 3)
	require.NoError(t, err)

	previousLeader, err := model.NewEvent(1, nil, nil)
	leaders := []*model.Event{previousLeader}
	require.NoError(t, err)
	for range 10 {
		event := ChainEvent(t, 1, 3, previousLeader)
		leaders = append(leaders, event)
		previousLeader = event
	}

	shuffledLeaders := slices.Clone(leaders)
	rand.Shuffle(len(shuffledLeaders), func(i, j int) {
		shuffledLeaders[i], shuffledLeaders[j] = shuffledLeaders[j], shuffledLeaders[i]
	})

	sortedLeaders, err := autocracy.SortLeaders(shuffledLeaders)
	require.NoError(t, err)
	require.Equal(t, leaders, sortedLeaders)
}

func ChainEvent(t *testing.T, creator model.CreatorId, repeat int, starting *model.Event) *model.Event {
	var selfParent, event *model.Event = starting, nil
	var err error
	for range repeat {
		var parents []*model.Event
		if selfParent == nil {
			parents = nil
		} else {
			parents = []*model.Event{selfParent}
		}
		event, err = model.NewEvent(creator, parents, nil)
		if err != nil {
			panic(err)
		}
		selfParent = event
	}
	return event
}
