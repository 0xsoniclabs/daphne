// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package dag

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/emitter"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testEmitInterval = 500 * time.Millisecond

var _ consensus.Factory = Factory[payload.Transactions]{}

func TestFactory_String_ProducesReadableSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	layering := layering.NewMockFactory(ctrl)
	layering.EXPECT().String().Return("test-layering").MinTimes(1)

	payloadFactory := payload.NewMockProtocolFactory[payload.Transactions](ctrl)
	payloadFactory.EXPECT().String().Return("test-payloads").MinTimes(1)

	emitterFactory := emitter.NewMockFactory(ctrl)
	emitterFactory.EXPECT().String().Return("test-emitter").MinTimes(1)

	factory := Factory[payload.Transactions]{
		EmitterFactory:         emitterFactory,
		LayeringFactory:        layering,
		PayloadProtocolFactory: payloadFactory,
	}
	require.Equal(t, "test-layering-test-payloads-test-emitter", factory.String())
}

func TestDagConsensus_NewActive_ActiveInstanceEmitsEvents(t *testing.T) {
	ctrl := gomock.NewController(t)

	const numEmissions = 5

	layeringProtocol := layering.NewMockLayering(ctrl)
	layeringProtocol.EXPECT().IsCandidate(gomock.Any()).Return(false).AnyTimes()
	layeringProtocol.EXPECT().SortLeaders(gomock.Len(0)).AnyTimes()
	layeringProtocol.EXPECT().GetRound(gomock.Any()).Return(uint32(0)).AnyTimes()

	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)
	payloadProtocol.EXPECT().BuildPayload(gomock.Any(), gomock.Any()).AnyTimes()

	lineup := txpool.NewMockLineup(ctrl)
	lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()

	transactionSource := consensus.NewMockTransactionProvider(ctrl)
	transactionSource.EXPECT().GetCandidateLineup().Return(lineup).Times(numEmissions)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().Times(numEmissions)
	server.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	synctest.Test(t, func(t *testing.T) {
		c := newActive(
			model.NewDag(consensus.NewUniformCommittee(1)),
			layeringProtocol,
			payloadProtocol,
			server,
			0,
			transactionSource,
			&emitter.PeriodicEmitterFactory{Interval: testEmitInterval})
		time.Sleep(numEmissions * testEmitInterval)
		c.Stop()
	})
}

func TestDagConsensus_processEventMessage_IgnoresAlreadySeenEvent(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus := newBase(model.NewDag(consensus.NewUniformCommittee(1)), layeringProtocol, payloadProtocol, server)

	event := EventMessage[payload.Transactions]{
		nested: model.EventMessage{Creator: 0},
	}
	// Only a single call to IsCandidate is made.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(false)
	layeringProtocol.EXPECT().SortLeaders(gomock.Len(0))
	// No calls to IsLeader are made.

	consensus.processEventMessage(event)
	consensus.processEventMessage(event)
}

func TestDagConsensus_processEventMessage_DiscardsNonCandidateEvents(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus := newBase(model.NewDag(consensus.NewUniformCommittee(1)), layeringProtocol, payloadProtocol, server)

	event := EventMessage[payload.Transactions]{
		nested: model.EventMessage{Creator: 0},
	}
	// The event is not a candidate.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(false)
	layeringProtocol.EXPECT().SortLeaders(gomock.Len(0))
	// No IsLeader calls are made.

	consensus.processEventMessage(event)
	// The event is not stored.
	require.Empty(t, consensus.leaderCandidates)
}

func TestDagConsensus_processEventMessage_MaintainsPotentialLeaders(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus := newBase(model.NewDag(consensus.NewUniformCommittee(1)), layeringProtocol, payloadProtocol, server)

	event := EventMessage[payload.Transactions]{}
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(true)
	// A call to IsLeader is made, and the event's leader status is reported as undecided.
	layeringProtocol.EXPECT().IsLeader(model.WithEventId(event.EventId())).Return(layering.VerdictUndecided)
	layeringProtocol.EXPECT().SortLeaders(gomock.Len(0))

	consensus.processEventMessage(event)
	// The event gets stored as a potential leader.
	require.Len(t, consensus.leaderCandidates, 1)
	require.Equal(t, event.EventId(), consensus.leaderCandidates[0].EventId())
}

func TestDagConsensus_processEventMessage_DeliversBundlesWhileMaintainingConsistentState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		layeringProtocol := layering.NewMockLayering(ctrl)

		payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)
		payloadProtocol.EXPECT().Merge(gomock.Any()).Return([]types.Bundle{{}}).AnyTimes()

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		server.EXPECT().GetPeers().AnyTimes()
		listener := consensus.NewMockBundleListener(ctrl)

		consensus := newBase(model.NewDag(consensus.NewUniformCommittee(2)), layeringProtocol, payloadProtocol, server)
		defer consensus.Stop()

		consensus.RegisterListener(listener)

		event1 := EventMessage[payload.Transactions]{nested: model.EventMessage{Creator: 0}}
		event2 := EventMessage[payload.Transactions]{nested: model.EventMessage{Creator: 1}}

		// Event 1 is initially a potential leader.
		layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event1.EventId())).Return(true)
		layeringProtocol.EXPECT().IsLeader(model.WithEventId(event1.EventId())).Return(layering.VerdictUndecided)
		layeringProtocol.EXPECT().SortLeaders(gomock.Len(0))
		consensus.processEventMessage(event1)

		// Event 2 is instantly a leader and "promotes" event 1 to a leader as well.
		layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event2.EventId())).Return(true)
		layeringProtocol.EXPECT().IsLeader(model.WithEventId(event1.EventId())).Return(layering.VerdictYes)
		layeringProtocol.EXPECT().IsLeader(model.WithEventId(event2.EventId())).Return(layering.VerdictYes)
		// SortLeaders should be called on both events.
		layeringProtocol.EXPECT().SortLeaders(gomock.Len(2)).Return([]*model.Event{{}, {}})

		// Both events should trigger the bundle listener.
		listener.EXPECT().OnNewBundle(gomock.Any()).Times(2)
		consensus.processEventMessage(event2)

		// Candidate evidence should end up empty.
		require.Empty(t, consensus.leaderCandidates)
		// The next bundle number should be incremented once for each event.
		require.Equal(t, uint32(2), consensus.nextBundleNumber)
	})
}

func TestDagConsensus_Stop_StopsEventEmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const numEmissions = 5

		layeringProtocol := layering.NewMockLayering(ctrl)
		layeringProtocol.EXPECT().IsCandidate(gomock.Any()).Return(false).AnyTimes()
		layeringProtocol.EXPECT().SortLeaders(gomock.Len(0)).AnyTimes()
		layeringProtocol.EXPECT().GetRound(gomock.Any()).Return(uint32(0)).AnyTimes()

		payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)
		payloadProtocol.EXPECT().BuildPayload(gomock.Any(), gomock.Any()).AnyTimes()

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()

		transactionSource := consensus.NewMockTransactionProvider(ctrl)
		transactionSource.EXPECT().GetCandidateLineup().Return(lineup).Times(numEmissions)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		// Allow some emissions to occur.
		server.EXPECT().GetPeers().Times(numEmissions)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

		c := newActive(
			model.NewDag(consensus.NewUniformCommittee(1)),
			layeringProtocol,
			payloadProtocol,
			server,
			1,
			transactionSource,
			&emitter.PeriodicEmitterFactory{Interval: testEmitInterval})
		time.Sleep(numEmissions * testEmitInterval)
		c.Stop()
		server.EXPECT().GetPeers().Times(0)
		// Wait to ensure no further emissions occur.
		time.Sleep(2 * testEmitInterval)
	})
}

func TestDagConsensus_Stop_StopsEventReceivingAndProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		server.EXPECT().GetPeers().AnyTimes()
		server.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

		layeringProtocol := layering.NewMockLayering(ctrl)
		payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)

		consensus := newPassive(model.NewDag(consensus.NewUniformCommittee(1)), layeringProtocol, payloadProtocol, server)

		// Expect first event to be processed.
		layeringProtocol.EXPECT().IsCandidate(gomock.Any()).Return(false)
		layeringProtocol.EXPECT().SortLeaders(gomock.Len(0))
		consensus.channel.Broadcast(EventMessage[payload.Transactions]{nested: model.EventMessage{Creator: 0}})
		// Notification of local listeners is asynchronous, so wait.
		synctest.Wait()

		// After stopping the consensus instance, received events should not
		// enter the processing pipeline.
		layeringProtocol.EXPECT().IsCandidate(gomock.Any()).Return(false).Times(0)
		consensus.Stop()
		// Different creator to ensure it's not considered a duplicate by a gossip.
		consensus.channel.Broadcast(EventMessage[payload.Transactions]{nested: model.EventMessage{Creator: 1}})
		synctest.Wait()
	})
}

func TestDagConsensus_createNewEvent_ForwardsMaximumRoundOfParentToPayloadProtocol(t *testing.T) {
	ctrl := gomock.NewController(t)

	selfParent := &model.Event{}
	eventA := &model.Event{}
	eventB := &model.Event{}

	selfRound := uint32(8)
	roundA := uint32(10)
	roundB := uint32(12)

	dag := model.NewMockDag(ctrl)
	dag.EXPECT().GetHeads().Return(
		map[consensus.ValidatorId]*model.Event{
			0: selfParent,
			1: eventA,
			2: eventB,
		},
	)

	layering := layering.NewMockLayering(ctrl)
	layering.EXPECT().GetRound(selfParent).Return(selfRound)
	layering.EXPECT().GetRound(eventA).Return(roundA)
	layering.EXPECT().GetRound(eventB).Return(roundB)

	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)
	payloadProtocol.EXPECT().BuildPayload(payload.EventMeta{
		ParentsMaxRound: max(selfRound, max(roundA, roundB)),
	}, gomock.Any())

	consensus := &Consensus[payload.Transactions]{
		dag:      dag,
		layering: layering,
		payloads: payloadProtocol,
	}
	consensus.createNewEvent(consensus.dag.GetHeads(), nil)
}

func TestDagConsensus_createNewEvent_ForwardsMaximumRoundOfParentToBeZeroForGenesisEvent(t *testing.T) {
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	dag.EXPECT().GetHeads().Return(nil)

	payloadProtocol := payload.NewMockProtocol[payload.Transactions](ctrl)
	payloadProtocol.EXPECT().BuildPayload(payload.EventMeta{
		ParentsMaxRound: 0,
	}, gomock.Any())

	consensus := &Consensus[payload.Transactions]{
		dag:      dag,
		payloads: payloadProtocol,
	}
	consensus.createNewEvent(consensus.dag.GetHeads(), nil)
}
