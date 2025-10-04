package model

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/stretchr/testify/require"
)

func TestDag_AddEvent_GenesisEventsAreImmediatelyConnected(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 1))

	genesisEvent := EventMessage{Creator: 1}

	connected := dag.AddEvent(genesisEvent)
	connectIds := make([]EventId, len(connected))
	for i, e := range connected {
		connectIds[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{genesisEvent.EventId()}, connectIds)
}

func TestDag_AddEvent_EventSequencesImmediatelyConnectedWhenAdded(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 1))

	eventMessage1 := EventMessage{Creator: 1}
	eventMessage2 := EventMessage{Creator: 1, Parents: []EventId{eventMessage1.EventId()}}

	connected1 := dag.AddEvent(eventMessage1)
	connectIds1 := make([]EventId, len(connected1))
	for i, e := range connected1 {
		connectIds1[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{eventMessage1.EventId()}, connectIds1)

	connected2 := dag.AddEvent(eventMessage2)
	connectIds2 := make([]EventId, len(connected2))
	for i, e := range connected2 {
		connectIds2[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{eventMessage2.EventId()}, connectIds2)
}

func TestDag_AddEvent_AddingInReverseDelaysConnection(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 1))

	eventMessage1 := EventMessage{Creator: 1}
	eventMessage2 := EventMessage{Creator: 1, Parents: []EventId{eventMessage1.EventId()}}

	connected1 := dag.AddEvent(eventMessage2)
	require.Empty(t, connected1, "Event should not be connected immediately")

	connected2 := dag.AddEvent(eventMessage1)
	connectIds2 := make([]EventId, len(connected2))
	for i, e := range connected2 {
		connectIds2[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{eventMessage1.EventId(), eventMessage2.EventId()}, connectIds2)
}

func TestDag_AddEvent_AddingDuplicateToPendingDoesNotDoAnything(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 1))

	// Random event that is not present in the DAG.
	// This ensures that events that have it as a parent will stay pending.
	randomId := EventId{1, 2, 3, 4, 5, 6, 7, 8}

	eventMessage := EventMessage{Creator: 1, Parents: []EventId{randomId}}

	connected1 := dag.AddEvent(eventMessage)
	require.Empty(t, connected1, "First addition should put it in pending")
	require.ElementsMatch(t, dag.pending, []EventMessage{eventMessage},
		"Pending events should contain the added event")

	connected2 := dag.AddEvent(eventMessage)
	require.Empty(t, connected2)
	require.ElementsMatch(t, dag.pending, []EventMessage{eventMessage},
		"Second addition should not change pending events")
}

func TestDag_AddEvent_CannotConnectAnEventTwice(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 1))

	eventMessage := EventMessage{Creator: 1}

	connected1 := dag.AddEvent(eventMessage)
	require.Len(t, connected1, 1, "First addition should connect the event")

	connected2 := dag.AddEvent(eventMessage)
	require.Empty(t, connected2,
		"Adding the same event again should not connect it again")
}

func TestDag_AddEvent_PanicOnInvalidEventMessage(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 2))
	parentEvent := EventMessage{Creator: 2}
	dag.AddEvent(parentEvent)

	// Create an event message with a first parent that is not the self-parent.
	eventMessage := EventMessage{Creator: 1, Parents: []EventId{parentEvent.EventId()}}

	// Expect panic when trying to add an event with invalid parent.
	require.Panics(t, func() {
		dag.AddEvent(eventMessage)
	}, "Adding an event with invalid parent should panic")
}

func TestDag_Getheads(t *testing.T) {
	dag := NewDag(newSimpleCommittee(t, 2))

	heads := dag.GetHeads()
	require.Empty(t, heads, "Initial heads should be empty")

	event1 := EventMessage{Creator: 1}
	event2 := EventMessage{Creator: 2}

	dag.AddEvent(event1)
	dag.AddEvent(event2)

	heads = dag.GetHeads()
	require.Len(t, heads, 2, "There should be two heads after adding two events")
	require.Equal(t, heads[1].seq, uint32(1), "Seq for creator 1 head should be 1")
	require.Equal(t, heads[2].seq, uint32(1), "Seq for creator 2 head should be 1")

	event3 := EventMessage{Creator: 1, Parents: []EventId{event1.EventId()}}
	dag.AddEvent(event3)

	heads = dag.GetHeads()
	require.Len(t, heads, 2, "There should still be two heads after adding event3")
	require.Equal(t, heads[1].seq, uint32(2), "Seq for creator 1 head should now be 2")
	require.Equal(t, heads[2].seq, uint32(1), "Seq for creator 2 head should still be 1")
}

func newSimpleCommittee(t *testing.T, size int) *consensus.Committee {
	t.Helper()
	committeeMap := map[consensus.ValidatorId]uint32{}
	for i := 1; i <= size; i++ {
		committeeMap[consensus.ValidatorId(i)] = 1
	}
	committee, err := consensus.NewCommittee(committeeMap)
	require.NoError(t, err)
	return committee
}
