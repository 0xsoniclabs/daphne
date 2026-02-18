// Copyright 2026 Sonic Operations Ltd
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

package central

import (
	"fmt"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testEmitInterval = 500 * time.Millisecond

var _ consensus.Factory = Factory{}

func TestFactory_String_ReturnsExpectedSummary(t *testing.T) {
	config := Factory{EmitInterval: 150 * time.Millisecond}
	require.Equal(t, "central-150ms", config.String())

	config = Factory{EmitInterval: 1 * time.Second}
	require.Equal(t, "central-1000ms", config.String())

	config = Factory{EmitInterval: 7499 * time.Microsecond}
	require.Equal(t, "central-7ms", config.String())

	config = Factory{EmitInterval: 7500 * time.Microsecond}
	require.Equal(t, "central-8ms", config.String())
}

func TestCentral_NewActive_InstantiatesActiveCentralAndRegistersListenerAndStartsEmittingBundles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leaderId := p2p.PeerId("leader")
		network := p2p.NewNetwork()
		server, err := network.NewServer(leaderId)
		require.NoError(t, err)

		const testInterval = testEmitInterval

		committee := consensus.NewUniformCommittee(1)
		leaderValidatorId := committee.GetHighestStakeValidator()
		config := Factory{
			EmitInterval: testInterval,
		}

		transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).MinTimes(1)

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

		centralConsensus := config.NewActive(server, *committee, leaderValidatorId, mockSource)
		centralConsensus.RegisterListener(mockListener)

		time.Sleep(2 * testInterval)
		centralConsensus.Stop()
	})
}

func TestCentral_NewActive_InstantiatesPassiveCentralIfNotCoordinatorAndDoesNotStartEmittingBundles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leaderId := p2p.PeerId("leader")
		network := p2p.NewNetwork()
		server, err := network.NewServer(leaderId)
		require.NoError(t, err)

		const testInterval = testEmitInterval

		committee := consensus.NewUniformCommittee(1)
		leaderValidatorId := committee.GetHighestStakeValidator()
		notLeadingValidatorId := consensus.ValidatorId(1)
		require.NotEqual(t, leaderValidatorId, notLeadingValidatorId)
		config := Factory{
			EmitInterval: testInterval,
		}

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		centralConsensus := config.NewActive(server, *committee, notLeadingValidatorId, mockSource)
		centralConsensus.RegisterListener(mockListener)

		time.Sleep(2 * testInterval)
		centralConsensus.Stop()
	})
}

func TestCentral_NewPassive_InstantiatesPassiveCentralAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	committee := consensus.NewUniformCommittee(1)
	config := Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)

	centralConsensus := config.NewPassive(server, *committee)
	centralConsensus.RegisterListener(mockListener)
}

func TestCentral_NewPassive_UsesProvidedBroadcastFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	channel := broadcast.NewMockChannel[BundleMessage](ctrl)
	channel.EXPECT().Register(gomock.Any()).Times(1)

	config := Factory{
		BroadcastFactory: func(
			server p2p.Server,
			extractKeyFromMessage func(BundleMessage) uint32,
		) broadcast.Channel[BundleMessage] {
			return channel
		},
	}

	centralConsensus := newPassiveCentral(server, &config)
	require.NotNil(t, centralConsensus)
	require.Equal(t, channel, centralConsensus.channel)
}

func TestCentral_NewActiveCentral_SetsEmitIntervalToDefaultIfNotSpecifiedAndStops(
	t *testing.T) {

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leaderId := p2p.PeerId("leader")
		network := p2p.NewNetwork()
		server, err := network.NewServer(leaderId)
		require.NoError(t, err)

		config := Factory{
			EmitInterval: 0,
		}

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).MinTimes(1)

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).Times(2)

		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

		centralConsensus := newActiveCentral(server, mockSource, &config)
		centralConsensus.RegisterListener(mockListener)

		time.Sleep(2 * testEmitInterval)
		centralConsensus.Stop()
	})
}

func TestCentral_HandleMessage_HandlesInvalidMessageCode(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	senderId := p2p.PeerId("sender")

	network := p2p.NewNetwork()
	leaderServer, err := network.NewServer(leaderId)
	require.NoError(t, err)
	senderServer, err := network.NewServer(senderId)
	require.NoError(t, err)

	config := Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)

	centralConsensus := newPassiveCentral(leaderServer, &config)
	centralConsensus.RegisterListener(mockListener)

	message := "ping"

	synctest.Test(t, func(t *testing.T) {
		err = senderServer.SendMessage(leaderId, message)
		require.NoError(t, err)
		synctest.Wait()
	})
}

func TestCentral_Broadcast_HandlesNetworkSendError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leader := p2p.PeerId("leader")
		peerId := p2p.PeerId("peer")
		mockServer := p2p.NewMockServer(ctrl)

		// Mock server returns a peer that will cause SendMessage to fail
		mockServer.EXPECT().GetLocalId().Return(leader).AnyTimes()
		mockServer.EXPECT().GetPeers().Return([]p2p.PeerId{peerId}).AnyTimes()
		mockServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

		// SendMessage will return an error to simulate network failure
		mockServer.EXPECT().SendMessage(peerId, gomock.Any()).
			Return(fmt.Errorf("network error")).AnyTimes()

		const testInterval = 100 * time.Millisecond

		committee := consensus.NewUniformCommittee(1)
		leaderValidatorId := committee.GetHighestStakeValidator()
		config := Factory{
			EmitInterval: testInterval,
		}

		transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).MinTimes(1)

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

		centralConsensus := config.NewActive(mockServer, *committee, leaderValidatorId, mockSource)
		centralConsensus.RegisterListener(mockListener)

		// Give time for bundle to be created and for broadcast to be attempted
		// (which will fail)
		time.Sleep(2 * testInterval)
		centralConsensus.Stop()
	})

}

func TestCentral_NewActiveCentral_EmitsBundlesInOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		peerId := p2p.PeerId("peer")
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
		server.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)
		server.EXPECT().GetPeers().Return([]p2p.PeerId{peerId}).AnyTimes()

		// Check the broadcasted bundles and their incrementing numbers
		next := uint32(0)
		server.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Do(
			func(peerId p2p.PeerId, msg p2p.Message) {
				bundle, ok := msg.(broadcast.GossipMessage[BundleMessage])
				require.True(t, ok, "unexpected message format")
				require.Equal(t, next, bundle.Payload.Bundle.Number, "unexpected bundle number")
				next++
			},
		).AnyTimes()

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().MinTimes(1)

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		const (
			emitInterval = 100 * time.Millisecond
			numCycles    = 5
		)

		centralConsensus := newActiveCentral(
			server,
			source,
			&Factory{EmitInterval: emitInterval},
		)
		time.Sleep(numCycles * emitInterval)
		centralConsensus.Stop()
		require.GreaterOrEqual(t, next, uint32(numCycles))
		require.Equal(t, next, centralConsensus.nextBundleNumber)
	})
}

func TestCentral_Stop_StopsBundleEmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		const numEmissions = 5

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().MinTimes(1)

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		// Allow some emissions to occur. 2 * numEmissions because each receival
		// triggers another broadcast.
		server.EXPECT().GetPeers().Times(2 * numEmissions)

		centralConsensus := newActiveCentral(
			server,
			source,
			&Factory{EmitInterval: testEmitInterval},
		)
		time.Sleep(numEmissions * testEmitInterval)

		centralConsensus.Stop()
		server.EXPECT().GetPeers().Times(0)
		// Wait to ensure no further emissions occur.
		time.Sleep(2 * testEmitInterval)
	})
}

func TestCentral_Stop_StopsBundleReceivingAndProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		consensus := newPassiveCentral(server, &Factory{EmitInterval: testEmitInterval})

		// A gossip broadcast should trigger a server send, and also trigger
		// a [Central.addBundle] call which should trigger another broadcast (and server send).
		// Thus the 2 expected calls to GetPeers.
		server.EXPECT().GetPeers().Times(2)
		consensus.channel.Broadcast(BundleMessage{})
		// Notification of local listeners is asynchronous, so wait.
		synctest.Wait()

		consensus.Stop()
		// After stopping the consensus instance, received bundles should not
		// enter the processing pipeline, meaning no further calls to GetPeers
		// by [Central.addBundle].
		server.EXPECT().GetPeers().Times(1)
		// Different bundle number to ensure it's not considered a duplicate by a gossip.
		consensus.channel.Broadcast(BundleMessage{Bundle: types.Bundle{Number: 2}})
		synctest.Wait()
	})
}

func TestBundleMessage_MessageSize_ReturnsCorrectSize(t *testing.T) {
	transactions := []types.Transaction{
		{From: 1, To: 2, Value: 10},
		{From: 3, To: 4, Value: 20},
	}
	sizes := make([]uint32, len(transactions))
	for i, tx := range transactions {
		sizes[i] = tx.MessageSize()
	}
	bundle := types.Bundle{
		Number:       1,
		Transactions: transactions,
	}
	bundleMsg := BundleMessage{Bundle: bundle}

	expectedSize := uint32(reflect.TypeFor[uint32]().Size()+
		reflect.TypeFor[types.Bundle]().Size()) + sizes[0] + sizes[1]

	actualSize := bundleMsg.MessageSize()

	require.Equal(t, expectedSize, actualSize)
}
