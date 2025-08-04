package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDag_GenesisEventsAreImmediatelyConnected(t *testing.T) {
	dag := NewDag()

	genesisEvent := EventMessage{Creator: 1}

	connected := dag.AddEvent(genesisEvent)
	connectIds := make([]EventId, len(connected))
	for i, e := range connected {
		connectIds[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{genesisEvent.EventId()}, connectIds)
}

func TestDag_AddingEventSequencesImmediatelyConnect(t *testing.T) {
	dag := NewDag()

	eventMessage1 := EventMessage{Creator: 1}
	eventMessage2 := EventMessage{Creator: 2, Parents: []EventId{eventMessage1.EventId()}}

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

func TestDag_AddingInReverseDelaysConnection(t *testing.T) {
	dag := NewDag()

	eventMessage1 := EventMessage{Creator: 1}
	eventMessage2 := EventMessage{Creator: 2, Parents: []EventId{eventMessage1.EventId()}}

	connected1 := dag.AddEvent(eventMessage2)
	require.Empty(t, connected1, "Event should not be connected immediately")

	connected2 := dag.AddEvent(eventMessage1)
	connectIds2 := make([]EventId, len(connected2))
	for i, e := range connected2 {
		connectIds2[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{eventMessage1.EventId(), eventMessage2.EventId()}, connectIds2)
}

func TestDag_GetClosure_SingleEventClosureForGenesisEvent(t *testing.T) {
	dag := NewDag()
	event := &Event{}
	require.ElementsMatch(t, dag.GetClosure(event), []*Event{event})
}

func TestDag_GetClosure_SimpleTreeClosure(t *testing.T) {
	dag := NewDag()

	closure := make([]*Event, 8)
	for i := range 8 {
		closure[i] = &Event{}
	}
	// e0 -> {e1 -> {e2, e3}, e4-> {e5->{e6, e7}}}
	closure[5].Parents = []*Event{closure[6], closure[7]}
	closure[4].Parents = []*Event{closure[5]}
	closure[1].Parents = []*Event{closure[2], closure[3]}
	closure[0].Parents = []*Event{closure[1], closure[4]}

	require.ElementsMatch(t, dag.GetClosure(closure[0]), closure)
}
