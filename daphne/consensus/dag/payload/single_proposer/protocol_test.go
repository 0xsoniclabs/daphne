package single_proposer

import "github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"

var _ payload.Protocol[SingleProposerPayload] = &SingleProposerProtocol{}
var _ payload.ProtocolFactory[SingleProposerPayload] = &SingleProposerProtocolFactory{}

/*
func TestCalculateIncomingSyncState_ProducesDefaultStateForGenesisEvent(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockEventReader(ctrl)

	event := &MutableEventPayload{}
	event.SetEpoch(42)
	require.Empty(event.Parents())

	state := CalculateIncomingProposalSyncState(world, event)
	want := ProposalSyncState{}
	require.Equal(want, state)
}

func TestCalculateIncomingSyncState_AggregatesParentStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockEventReader(ctrl)

	p1 := hash.Event{1}
	p2 := hash.Event{2}
	p3 := hash.Event{3}
	parents := map[hash.Event]Payload{
		p1: {ProposalSyncState: ProposalSyncState{
			LastSeenProposalTurn:  Turn(0x01),
			LastSeenProposalFrame: idx.Frame(0x12),
		}},
		p2: {ProposalSyncState: ProposalSyncState{
			LastSeenProposalTurn:  Turn(0x03),
			LastSeenProposalFrame: idx.Frame(0x11),
		}},
		p3: {ProposalSyncState: ProposalSyncState{
			LastSeenProposalTurn:  Turn(0x02),
			LastSeenProposalFrame: idx.Frame(0x13),
		}},
	}

	world.EXPECT().GetEventPayload(p1).Return(parents[p1])
	world.EXPECT().GetEventPayload(p2).Return(parents[p2])
	world.EXPECT().GetEventPayload(p3).Return(parents[p3])

	event := &dag.MutableBaseEvent{}
	event.SetParents(hash.Events{p1, p2, p3})
	state := CalculateIncomingProposalSyncState(world, event)

	require.Equal(Turn(0x03), state.LastSeenProposalTurn)
	require.Equal(idx.Frame(0x13), state.LastSeenProposalFrame)
}
*/
