package model

import (
	"reflect"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
			parents:     []*Event{{seq: 1, creator: consensus.ValidatorId(1)}},
			expectedSeq: 2,
		},
		"self-parent and other parents": {
			parents: []*Event{
				{seq: 2, creator: consensus.ValidatorId(1)},
				{creator: consensus.ValidatorId(2)},
			},
			expectedSeq: 3,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			event, err := NewEvent(consensus.ValidatorId(1), testCase.parents, []types.Transaction{})
			require.NoError(t, err)
			require.Equal(t, testCase.expectedSeq, event.Seq())
		})
	}
}

func TestEvent_Creator(t *testing.T) {
	parents := []*Event{
		{creator: consensus.ValidatorId(2)},
	}
	payload := []types.Transaction{}
	event, _ := NewEvent(consensus.ValidatorId(2), parents, payload)
	require.Equal(t, consensus.ValidatorId(2), event.Creator(), "Creator should return the correct creator ID")
}

func TestEvent_Parents(t *testing.T) {
	event := &Event{
		parents: []*Event{
			{creator: consensus.ValidatorId(1)},
			{creator: consensus.ValidatorId(2)},
		},
	}
	parents := event.Parents()
	require.Len(t, parents, 2, "Parents should return a slice with the correct length")
	require.Equal(t, consensus.ValidatorId(1), parents[0].creator, "First parent should match the expected creator ID")
	require.Equal(t, consensus.ValidatorId(2), parents[1].creator, "Second parent should match the expected creator ID")
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
	creator := consensus.ValidatorId(1)
	parents := []*Event{
		{creator: creator},
		{creator: consensus.ValidatorId(2)},
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
	creator := consensus.ValidatorId(1)
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
	creator := consensus.ValidatorId(1)
	parents := []*Event{
		{creator: 2}, // This parent is not created by the same validator
		{creator: creator},
	}
	payload := []types.Transaction{}

	event, err := NewEvent(creator, parents, payload)
	require.Error(t, err, "NewEvent should return an error for non-self first parent")
	require.Nil(t, event, "Event should be nil when an error occurs")
}

func TestEventId_String(t *testing.T) {
	tests := map[EventId]string{
		{}:                                       "0000000000000000",
		{1, 2, 3}:                                "0102030000000000",
		EventId(slices.Repeat([]byte{0x25}, 32)): "2525252525252525",
	}

	for eventId, expected := range tests {
		t.Run(expected, func(t *testing.T) {
			require.Equal(t, eventId.String(), expected)
		})
	}
}

func TestEvent_EventId_EveryEventHasAUniqueId(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		creator consensus.ValidatorId
		parents []consensus.ValidatorId
	}{
		{creator: 1},
		{creator: 2},
		{creator: 1, parents: []consensus.ValidatorId{1}},
		{creator: 1, parents: []consensus.ValidatorId{1, 2}},
		{creator: 1, parents: []consensus.ValidatorId{1, 3}},
	}
	events := []*Event{}
	for _, tt := range tests {
		parents := []*Event{}
		for _, parentId := range tt.parents {
			parent, err := NewEvent(parentId, []*Event{}, []types.Transaction{})
			require.NoError(err)

			parents = append(parents, parent)
		}
		event, err := NewEvent(tt.creator, parents, []types.Transaction{})
		require.NoError(err)

		events = append(events, event)
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

func TestEvent_TraverseClosure_VisitsAllEventsOnceSimpleTreeClosure(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := make([]*Event, 8)
	for i := range 8 {
		events[i] = &Event{}
	}
	// e0 -> {e1 -> {e2, e3}, e4-> {e5->{e6, e7}}}
	// No event has multiple events pointed to it.
	events[5].parents = []*Event{events[6], events[7]}
	events[4].parents = []*Event{events[5]}
	events[1].parents = []*Event{events[2], events[3]}
	events[0].parents = []*Event{events[1], events[4]}

	visitor := NewMockEventVisitor(ctrl)
	for _, event := range events {
		visitor.EXPECT().Visit(event).Return(Visit_Descent)
	}

	events[0].TraverseClosure(visitor)
}

func TestEvent_TraverseClosure_VisitsAllEventsOnceOverlappingTreeClosure(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := make([]*Event, 4)
	for i := range 4 {
		events[i] = &Event{}
	}
	// e0 -> {e1 -> {e3}, e2-> {e3}}
	// e3 is pointed to by two parents, but should only appear once in the closure.
	events[0].parents = []*Event{events[1], events[2]}
	events[1].parents = []*Event{events[3]}
	events[2].parents = []*Event{events[3]}
	events[3].parents = []*Event{}

	visitor := NewMockEventVisitor(ctrl)
	for _, event := range events {
		visitor.EXPECT().Visit(event).Return(Visit_Descent)
	}

	events[0].TraverseClosure(visitor)
}

func TestEvent_TraverseClosure_PrunesBranchOnVisitPruneSignal(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := make([]*Event, 8)
	for i := range 8 {
		events[i] = &Event{}
	}
	// e0 -> {e1 -> {e2, e3}, e4-> {e5->{e6, e7}}}
	events[5].parents = []*Event{events[6], events[7]}
	events[4].parents = []*Event{events[5]}
	events[1].parents = []*Event{events[2], events[3]}
	events[0].parents = []*Event{events[1], events[4]}

	visitor := NewMockEventVisitor(ctrl)
	visitor.EXPECT().Visit(events[4]).Return(Visit_Prune)
	for _, event := range events[:4] {
		visitor.EXPECT().Visit(event).Return(Visit_Descent)
	}
	// No visits for events e5, e6, e7 are expected as the branch is pruned.
	events[0].TraverseClosure(visitor)
}

func TestEvent_TraverseClosure_AbortsTraversalOnVisitAbortSignal(t *testing.T) {
	ctrl := gomock.NewController(t)

	events := make([]*Event, 8)
	for i := range 8 {
		events[i] = &Event{}
	}
	// e0 -> {e1 -> {e2, e3}, e4-> {e5->{e6, e7}}}
	events[5].parents = []*Event{events[6], events[7]}
	events[4].parents = []*Event{events[5]}
	events[1].parents = []*Event{events[2], events[3]}
	events[0].parents = []*Event{events[1], events[4]}

	visitor := NewMockEventVisitor(ctrl)
	visitor.EXPECT().Visit(events[1]).Return(Visit_Abort)
	for _, event := range events[:1] {
		visitor.EXPECT().Visit(event).Return(Visit_Descent)
	}
	// Only events e0 and e1 are visited, then traversal aborts for all
	// pending branches.
	events[0].TraverseClosure(visitor)
}

func TestEventVisitor_WrapEventVisitor_CallsProvidedMethodOnVisit(t *testing.T) {
	ctrl := gomock.NewController(t)

	event := &Event{}

	// Using the mocked visitor for convenience of verifying the call.
	visitor := NewMockEventVisitor(ctrl)
	visitor.EXPECT().Visit(event)

	wrappedVisitor := WrapEventVisitor(func(e *Event) VisitResult {
		return visitor.Visit(e)
	})

	event.TraverseClosure(wrappedVisitor)
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
		consensus.ValidatorId(1), []*Event{{creator: consensus.ValidatorId(1)}, {creator: consensus.ValidatorId(2)}}, transactions,
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

func TestEventMessage_MessageSize(t *testing.T) {
	transactions := []types.Transaction{
		{
			From:  1,
			To:    2,
			Value: 100,
			Nonce: 5,
		},
		{
			From:  3,
			To:    4,
			Value: 200,
			Nonce: 10,
		},
	}
	sizes := make([]uint32, len(transactions))
	for i, tx := range transactions {
		sizes[i] = tx.MessageSize()
	}
	eventMessage := EventMessage{
		Creator: consensus.ValidatorId(1),
		Parents: []EventId{
			{1, 2, 3},
			{4, 5, 6},
		},
		Payload: transactions,
	}

	expectedSize := uint32(reflect.TypeFor[EventMessage]().Size()) +
		2*uint32(reflect.TypeFor[EventId]().Size()) +
		sizes[0] + sizes[1]

	actualSize := eventMessage.MessageSize()

	require.Equal(t, expectedSize, actualSize,
		"EventMessage MessageSize should return the correct size")
}
