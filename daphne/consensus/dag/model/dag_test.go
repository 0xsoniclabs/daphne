package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDag_GenesisEventsAreImmediatelyConnected(t *testing.T) {
	store := &Store{}
	dag := &Dag{store: store}

	genesisEvent := Event{Creator: 1}

	connected := dag.AddEvent(genesisEvent)
	require.ElementsMatch(t, []EventId{genesisEvent.EventId()}, connected)
}

func TestDag_AddingEventSequencesImmediatelyConnect(t *testing.T) {
	store := &Store{}
	dag := &Dag{store: store}

	event1 := Event{Creator: 1}
	event2 := Event{Creator: 2, Parents: []EventId{event1.EventId()}}

	connected1 := dag.AddEvent(event1)
	require.ElementsMatch(t, []EventId{event1.EventId()}, connected1)

	connected2 := dag.AddEvent(event2)
	require.ElementsMatch(t, []EventId{event2.EventId()}, connected2)
}

func TestDag_AddingInReverseDelaysConnection(t *testing.T) {
	store := &Store{}
	dag := &Dag{store: store}

	event1 := Event{Creator: 1}
	event2 := Event{Creator: 2, Parents: []EventId{event1.EventId()}}

	connected1 := dag.AddEvent(event2)
	require.Empty(t, connected1, "Event should not be connected immediately")

	connected2 := dag.AddEvent(event1)
	require.ElementsMatch(t, []EventId{event1.EventId(), event2.EventId()}, connected2)
}
