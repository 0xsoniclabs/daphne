package model

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestEvent_Seq(t *testing.T) {
	tests := map[string]struct {
		parents     []*Event
		expectedSeq uint32
	}{
		"genesis event": {
			parents:     []*Event{},
			expectedSeq: 1,
		},
		"single self-parent": {
			parents:     []*Event{{seq: 1, creator: CreatorId(1)}},
			expectedSeq: 2,
		},
		"self-parent and other parents": {
			parents:     []*Event{{seq: 2, creator: CreatorId(1)}, {creator: CreatorId(2)}},
			expectedSeq: 3,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			event, err := NewEvent(CreatorId(1), testCase.parents, []types.Transaction{})
			require.NoError(t, err)
			require.Equal(t, testCase.expectedSeq, event.Seq())
		})
	}
}

func TestEvent_Creator(t *testing.T) {
	parents := []*Event{
		{creator: CreatorId(2)},
	}
	payload := []types.Transaction{}
	event, _ := NewEvent(CreatorId(2), parents, payload)
	require.Equal(t, CreatorId(2), event.Creator(), "Creator should return the correct creator ID")
}

func TestEvent_Parents(t *testing.T) {
	event := &Event{
		parents: []*Event{
			{creator: CreatorId(1)},
			{creator: CreatorId(2)},
		},
	}
	parents := event.Parents()
	require.Len(t, parents, 2, "Parents should return a slice with the correct length")
	require.Equal(t, CreatorId(1), parents[0].creator, "First parent should match the expected creator ID")
	require.Equal(t, CreatorId(2), parents[1].creator, "Second parent should match the expected creator ID")
	require.NotSame(t, &event.parents, &parents, "Parents should return a copy of the slice, not the original")
}

func TestEvent_Payload(t *testing.T) {
	event := &Event{
		payload: []types.Transaction{
			{From: 1, To: 2, Value: 100, Nonce: 5},
			{From: 3, To: 4, Value: 200, Nonce: 10},
		},
	}
	payload := event.Payload()
	require.Len(t, payload, 2,
		"Payload should return a slice with the correct length")
	require.Equal(t, types.Transaction{From: 1, To: 2, Value: 100, Nonce: 5}, payload[0],
		"First transaction should match the expected values")
	require.Equal(t, types.Transaction{From: 3, To: 4, Value: 200, Nonce: 10}, payload[1],
		"Second transaction should match the expected values")
	require.NotSame(t, &event.payload, &payload,
		"Payload should return a copy of the slice, not the original")
}

func TestEvent_NewEvent_CreatesEventWithValidParameters(t *testing.T) {
	creator := CreatorId(1)
	parents := []*Event{
		{creator: creator},
		{creator: 2},
	}
	payload := []types.Transaction{}

	event, err := NewEvent(creator, parents, payload)
	require.NoError(t, err, "NewEvent should not return an error")
	require.NotNil(t, event, "NewEvent should return a valid Event instance")
	require.Equal(t, creator, event.creator, "Event creator should match the provided creator")
	require.Equal(t, parents, event.parents, "Event parents should match the provided parents")
	require.Equal(t, payload, event.payload, "Event payload should match the provided payload")
}

func TestEvent_NewEvent_FailsWithNilParent(t *testing.T) {
	creator := CreatorId(1)
	parents := []*Event{
		{creator: creator},
		nil,
	}
	payload := []types.Transaction{}

	event, err := NewEvent(creator, parents, payload)
	require.Error(t, err, "NewEvent should return an error for nil parent")
	require.Nil(t, event, "Event should be nil when an error occurs")
}

func TestEvent_NewEvent_FailsWithFirstParentFromAnotherCreator(t *testing.T) {
	creator := CreatorId(1)
	parents := []*Event{
		{creator: 2}, // This parent is not created by the same validator
		{creator: creator},
	}
	payload := []types.Transaction{}

	event, err := NewEvent(creator, parents, payload)
	require.Error(t, err, "NewEvent should return an error for non-self first parent")
	require.Nil(t, event, "Event should be nil when an error occurs")
}

func TestEvent_EventId_EveryEventHasAUniqueId(t *testing.T) {
	events := []Event{
		{creator: 1},
		{creator: 2},
		{creator: 1, parents: []*Event{{creator: 1}}},
		{creator: 1, parents: []*Event{{creator: 1}, {creator: 2}}},
		{creator: 1, parents: []*Event{{creator: 1}, {creator: 3}}},
	}

	for i, event := range events {
		id := event.EventId()
		for j, other := range events {
			if i == j {
				continue
			}
			require.NotEqual(t, id, other.EventId(), "Event %d should not have the same ID as Event %d", i, j)
		}
	}
}

func TestEvent_GenesisEvent_HasNoSelfParent(t *testing.T) {
	event := Event{creator: 1}
	require.Nil(t, event.SelfParent(), "Genesis event should not have a self parent")
}

func TestEvent_FirstParentIsSelfParent(t *testing.T) {
	firstParent := &Event{creator: 1}
	event := Event{
		creator: 1,
		parents: []*Event{firstParent, {creator: 2}},
	}
	selfParent := event.SelfParent()
	require.NotNil(t, selfParent, "Event should have a self parent")
	require.Equal(t, selfParent.EventId(), firstParent.EventId(), "First parent should be the self parent")
}

func TestEvent_IsGenesis_ReturnsFalseIfThereAreParentedEvents(t *testing.T) {
	event := Event{creator: 1}
	require.True(t, event.IsGenesis(), "Event with no parents should be a genesis event")

	eventWithParent := Event{
		creator: 1,
		parents: []*Event{{creator: 2}},
	}
	require.False(t, eventWithParent.IsGenesis(), "Event with parents should not be a genesis event")
}

func TestDag_GetClosure_SingleEventClosureForGenesisEvent(t *testing.T) {
	event := &Event{}
	set := map[*Event]struct{}{
		event: {},
	}
	require.Equal(t, event.GetClosure(), set)
}

func TestDag_GetClosure_SimpleTreeClosure(t *testing.T) {
	events := make([]*Event, 8)
	for i := range 8 {
		events[i] = &Event{}
	}
	// e0 -> {e1 -> {e2, e3}, e4-> {e5->{e6, e7}}}
	events[5].parents = []*Event{events[6], events[7]}
	events[4].parents = []*Event{events[5]}
	events[1].parents = []*Event{events[2], events[3]}
	events[0].parents = []*Event{events[1], events[4]}

	set := map[*Event]struct{}{
		events[0]: {},
		events[1]: {},
		events[2]: {},
		events[3]: {},
		events[4]: {},
		events[5]: {},
		events[6]: {},
		events[7]: {},
	}

	require.Equal(t, events[0].GetClosure(), set)
}

func TestDag_GetClosure_AvoidsDuplicates(t *testing.T) {
	events := make([]*Event, 8)
	for i := range 4 {
		events[i] = &Event{}
	}
	events[0].parents = []*Event{events[1], events[2]}
	events[1].parents = []*Event{events[3]}
	events[2].parents = []*Event{events[3]}
	events[3].parents = []*Event{}
	set := map[*Event]struct{}{
		events[0]: {},
		events[1]: {},
		events[2]: {},
		events[3]: {},
	}
	require.Equal(t, events[0].GetClosure(), set,
		"Closure should not contain duplicates, even if multiple parents point to the same event")
}

func TestEvent_ToEventMessage(t *testing.T) {
	transactions := []types.Transaction{
		{
			From:  1,
			To:    2,
			Value: 100,
			Nonce: 5,
		},
	}

	event, _ := NewEvent(
		CreatorId(1), []*Event{{creator: 1}, {creator: 2}}, transactions,
	)

	msg := event.ToEventMessage()

	require.Equal(t, event.creator, msg.Creator,
		"EventMessage creator should match Event creator")
	require.Len(t, msg.Parents, 2, "EventMessage should have two parents")
	for i, parent := range msg.Parents {
		require.Equal(t, event.parents[i].EventId(), parent,
			"Parent %d of EventMessage should match corresponding Event parent", i)
	}
	require.Equal(t, transactions, msg.Payload)
}
