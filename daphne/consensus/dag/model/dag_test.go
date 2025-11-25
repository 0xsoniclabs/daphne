package model

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/stretchr/testify/require"
)

func TestDag_AddEvent_GenesisEventsAreImmediatelyConnected(t *testing.T) {
	dag := newDag(consensus.NewUniformCommittee(1))

	genesisEvent := EventMessage{Creator: 0}

	connected := dag.AddEvent(genesisEvent)
	connectIds := make([]EventId, len(connected))
	for i, e := range connected {
		connectIds[i] = e.EventId()
	}
	require.ElementsMatch(t, []EventId{genesisEvent.EventId()}, connectIds)
}

func TestDag_AddEvent_EventSequencesImmediatelyConnectedWhenAdded(t *testing.T) {
	dag := newDag(consensus.NewUniformCommittee(2))

	eventMessage1 := EventMessage{Creator: 0}
	eventMessage2 := EventMessage{Creator: 0, Parents: []EventId{eventMessage1.EventId()}}

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
	dag := newDag(consensus.NewUniformCommittee(1))

	eventMessage1 := EventMessage{Creator: 0}
	eventMessage2 := EventMessage{Creator: 0, Parents: []EventId{eventMessage1.EventId()}}

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
	dag := newDag(consensus.NewUniformCommittee(1))

	// Random event that is not present in the DAG.
	// This ensures that events that have it as a parent will stay pending.
	randomId := EventId{1, 2, 3, 4, 5, 6, 7, 8}

	eventMessage := EventMessage{Creator: 0, Parents: []EventId{randomId}}

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
	dag := newDag(consensus.NewUniformCommittee(1))

	eventMessage := EventMessage{Creator: 0}

	connected1 := dag.AddEvent(eventMessage)
	require.Len(t, connected1, 1, "First addition should connect the event")

	connected2 := dag.AddEvent(eventMessage)
	require.Empty(t, connected2,
		"Adding the same event again should not connect it again")
}

func TestDag_AddEvent_PanicOnInvalidEventMessage(t *testing.T) {
	dag := newDag(consensus.NewUniformCommittee(2))
	parentEvent := EventMessage{Creator: 1}
	dag.AddEvent(parentEvent)

	// Create an event message with a first parent that is not the self-parent.
	eventMessage := EventMessage{Creator: 0, Parents: []EventId{parentEvent.EventId()}}

	// Expect panic when trying to add an event with invalid parent.
	require.Panics(t, func() {
		dag.AddEvent(eventMessage)
	}, "Adding an event with invalid parent should panic")
}

func TestDag_AddEvent_UpdatesHighestbeforeAndLowestOnlyOnConnection(t *testing.T) {
	require := require.New(t)

	dag := newDag(consensus.NewUniformCommittee(2))

	eventMessage1 := EventMessage{Creator: 0}
	eventMessage2 := EventMessage{Creator: 0, Parents: []EventId{eventMessage1.EventId()}}

	// Should not modify highestBefore as no connections occurred yet.
	require.Empty(dag.AddEvent(eventMessage2))
	require.Empty(dag.highestBefore)
	require.Empty(dag.lowestAfter)

	connected := dag.AddEvent(eventMessage1)
	require.Len(connected, 2)
	require.Equal(uint32(1), dag.highestBefore[connected[0]][0])
	require.Equal(uint32(2), dag.highestBefore[connected[1]][0])
	require.Equal(uint32(1), dag.lowestAfter[connected[0]][0])
	require.Equal(uint32(2), dag.lowestAfter[connected[1]][0])
}

func TestDag_updateHighestBefore(t *testing.T) {
	require := require.New(t)

	dag := newDag(consensus.NewUniformCommittee(2))

	// e_#creatorid_#seq

	e0_1 := &Event{seq: 1, creator: 0}
	e0_2 := &Event{seq: 2, creator: 0, parents: []*Event{e0_1}}

	// Both should have themselves as their highestBefore for creator 0.
	dag.updateHighestBefore(e0_1)
	require.Equal(uint32(1), dag.highestBefore[e0_1][0])

	dag.updateHighestBefore(e0_2)
	require.Equal(uint32(2), dag.highestBefore[e0_2][0])

	e1_1 := &Event{seq: 1, creator: 1}
	// e1_2 references the lower event from creator 0.
	e1_2 := &Event{seq: 2, creator: 1, parents: []*Event{e1_1, e0_1}}
	// e1_3 references the higher event from creator 0 and its self-parent.
	e1_3 := &Event{seq: 3, creator: 1, parents: []*Event{e1_2, e0_2}}

	dag.updateHighestBefore(e1_1)
	require.Equal(uint32(1), dag.highestBefore[e1_1][1])
	// e1_1 should not have any highestBefore from creator 0 yet.
	require.Equal(uint32(0), dag.highestBefore[e1_1][0])

	dag.updateHighestBefore(e1_2)
	require.Equal(uint32(2), dag.highestBefore[e1_2][1])
	// e1_2 should now have highestBefore from creator 0 as e0_1 (seq = 1).
	require.Equal(uint32(1), dag.highestBefore[e1_2][0])

	dag.updateHighestBefore(e1_3)
	require.Equal(uint32(3), dag.highestBefore[e1_3][1])
	// e1_3 should should take the max highestBefore from its parents for creator 0,
	// which is 2 (e1_2 having seq=1 and e0_2 having seq=2 for creator 0).
	require.Equal(uint32(2), dag.highestBefore[e1_3][0])
}

func TestDag_updateLowestAfter(t *testing.T) {
	require := require.New(t)

	dag := newDag(consensus.NewUniformCommittee(2))

	// e_#creatorid_#seq

	e0_1 := &Event{seq: 1, creator: 0}
	e0_2 := &Event{seq: 2, creator: 0, parents: []*Event{e0_1}}

	// Both should have themselves as their lowestAfter for creator 0.
	dag.updateLowestAfter(e0_1)
	require.Equal(uint32(1), dag.lowestAfter[e0_1][0])

	dag.updateLowestAfter(e0_2)
	require.Equal(uint32(2), dag.lowestAfter[e0_2][0])

	e1_1 := &Event{seq: 1, creator: 1}
	// e1_2 references the lower event from creator 0.
	e1_2 := &Event{seq: 2, creator: 1, parents: []*Event{e1_1, e0_1}}
	// e1_3 references the higher event from creator 0.
	e1_3 := &Event{seq: 3, creator: 1, parents: []*Event{e1_2, e0_2}}

	dag.updateLowestAfter(e1_1)
	require.Equal(uint32(1), dag.lowestAfter[e1_1][1])

	dag.updateLowestAfter(e1_2)
	require.Equal(uint32(2), dag.lowestAfter[e1_2][1])
	// e0_1's lowestAfter event should be e1_2 (seq = 2).
	require.Equal(uint32(2), dag.lowestAfter[e0_1][1])
	// while e0_2 has no lowestAfter events from creator 1 yet.
	require.Equal(uint32(0), dag.lowestAfter[e0_2][1])

	dag.updateLowestAfter(e1_3)
	require.Equal(uint32(3), dag.lowestAfter[e1_3][1])
	// e0_2's lowestAfter should now be updated to e1_3 (seq = 3).
	require.Equal(uint32(3), dag.lowestAfter[e0_2][1])
	// e0_1 lowestAfter should remain unchanged.
	require.Equal(uint32(2), dag.lowestAfter[e0_1][1])
}

func TestDag_GetHeads(t *testing.T) {
	dag := newDag(consensus.NewUniformCommittee(2))

	heads := dag.GetHeads()
	require.Empty(t, heads, "Initial heads should be empty")

	event1 := EventMessage{Creator: 0}
	event2 := EventMessage{Creator: 1}

	dag.AddEvent(event1)
	dag.AddEvent(event2)

	heads = dag.GetHeads()
	require.Len(t, heads, 2, "There should be two heads after adding two events")
	require.Equal(t, heads[0].seq, uint32(1), "Seq for creator 1 head should be 1")
	require.Equal(t, heads[1].seq, uint32(1), "Seq for creator 2 head should be 1")

	event3 := EventMessage{Creator: 0, Parents: []EventId{event1.EventId()}}
	dag.AddEvent(event3)

	heads = dag.GetHeads()
	require.Len(t, heads, 2, "There should still be two heads after adding event3")
	require.Equal(t, heads[0].seq, uint32(2), "Seq for creator 1 head should now be 2")
	require.Equal(t, heads[1].seq, uint32(1), "Seq for creator 2 head should still be 1")
}

func TestDag_AddEvent_IgnoresEventsFromNonCommitteeMembers(t *testing.T) {
	dag := NewDag(consensus.NewUniformCommittee(1))

	// Parentless event from a non-committee member (creator 2), should be rejected.
	connected := dag.AddEvent(EventMessage{Creator: 2})
	require.Empty(t, connected)
}

func TestDag_reachability_ReturnsFalseForNonConnectedEvents(t *testing.T) {
	committee := consensus.NewUniformCommittee(2)
	dag := NewDag(committee)

	event1 := createEventAndAddToDag(t, dag, 0, nil)
	event2 := &Event{seq: 1, creator: 1} // Not added to DAG

	require.False(t, dag.Reaches(event1, event2))
	require.False(t, dag.Reaches(event2, event1))
	require.False(t, dag.StronglyReaches(event1, event2))
	require.False(t, dag.StronglyReaches(event2, event1))
}

func TestLachesis_reachability_stepTopologyWithOddTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(11)

	testDag_reachability_stepTopology(t, committee)
}

func TestDag_reachability_stepTopologyWithEvenTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(12)

	testDag_reachability_stepTopology(t, committee)
}

func testDag_reachability_stepTopology(t *testing.T, committee *consensus.Committee) {
	require := require.New(t)
	dag := NewDag(committee)

	//     An example step topology with 4 creators.
	//
	//              e_#creatorid_#seq
	//
	//                            ╬═══════════e_4_2
	//                            ║             ║
	//  			  ╬═════════e_3_2           ║
	//                ║           ║             ║
	// ╬════════════e_2_2         ║			    ║
	// ║              ║           ║             ║
	// e_1_2		  ║		      ║             ║
	// ║              ║           ║             ║
	// e_1_1        e_2_1       e_3_1         e_4_1

	genesisEvents := make([]*Event, 0, len(committee.Validators()))
	for _, validatorId := range committee.Validators() {
		genesisEvent := createEventAndAddToDag(t, dag, validatorId, nil)
		genesisEvents = append(genesisEvents, genesisEvent)
	}

	// Target event is the first creator genesis event (leftmost in the diagram).
	targetEvent := genesisEvents[0]

	var nonSelfParent *Event = nil
	// Build the topology from left to right, where each event has a self-parent
	// and a non-self-parent which is the last event created by a validator to the left.
	for _, validatorId := range committee.Validators() {
		t.Run(fmt.Sprint("step ", validatorId), func(t *testing.T) {
			parents := []*Event{genesisEvents[validatorId]}
			if nonSelfParent != nil {
				parents = append(parents, nonSelfParent)
			}
			event := createEventAndAddToDag(t, dag, validatorId, parents)

			expected := int(validatorId)+1 >= len(committee.Validators())*2/3+1
			require.Equal(expected, dag.StronglyReaches(event, targetEvent))

			reaches := func(source, target *Event) bool {
				targetFound := false
				source.TraverseClosure(WrapEventVisitor(func(e *Event) VisitResult {
					if e == target {
						targetFound = true
						return Visit_Abort
					}
					return Visit_Descent
				}))
				return targetFound
			}
			// Verify reachability from the new event to all genesis events.
			for _, genesis := range genesisEvents {
				require.Equal(reaches(event, genesis), dag.Reaches(event, genesis))
			}

			nonSelfParent = event
		})
	}
}

func createEventAndAddToDag(t *testing.T, dag Dag, creator consensus.ValidatorId, parents []*Event) *Event {
	t.Helper()

	parentIds := make([]EventId, 0, len(parents))
	for _, parent := range parents {
		parentIds = append(parentIds, parent.EventId())
	}
	newEvents := dag.AddEvent(EventMessage{Creator: creator, Parents: parentIds})
	require.Len(t, newEvents, 1)
	require.NotNil(t, newEvents[0])

	return newEvents[0]
}
