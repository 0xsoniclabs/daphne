package central

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCentral_NewActive_InstantiatesActiveCentralAndRegistersListenerAndStartsEmittingBundles(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	const testInterval = generic.DefaultEmitInterval

	config := Factory{
		EmitInterval: testInterval,
		Leader:       leaderId,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	centralConsensus := config.NewActive(server, mockSource)
	centralConsensus.RegisterListener(mockListener)

	time.Sleep(2 * testInterval)
}

func TestCentral_NewActive_InstantiatesPassiveCentralIfNotCoordinatorAndDoesNotStartEmittingBundles(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	const testInterval = generic.DefaultEmitInterval

	config := Factory{
		EmitInterval: testInterval,
		Leader:       p2p.PeerId("not-leader"),
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).Times(0)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

	centralConsensus := config.NewActive(server, mockSource)
	centralConsensus.RegisterListener(mockListener)

	time.Sleep(2 * testInterval)
}

func TestCentral_NewPassive_InstantiatesPassiveCentralAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)

	centralConsensus := config.NewPassive(server)
	centralConsensus.RegisterListener(mockListener)
}

func TestCentral_NewActiveCentral_SetsEmitIntervalToDefaultIfNotSpecifiedAndStops(
	t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := Factory{
		EmitInterval: 0,
	}

	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().
		Return([]types.Transaction{}).MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	centralConsensus := newActiveCentral(server, mockSource, &config)
	centralConsensus.RegisterListener(mockListener)
	defer centralConsensus.Stop()

	time.Sleep(2 * generic.DefaultEmitInterval)
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

	message := p2p.Message{
		Code: p2p.MessageCode_UnitTestProtocol_Ping,
	}

	synctest.Test(t, func(t *testing.T) {
		err = senderServer.SendMessage(leaderId, message)
		require.NoError(t, err)
		synctest.Wait()
	})
}

func TestCentral_HandleMessage_HandlesInvalidBundlePayload(t *testing.T) {
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

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: "invalid-payload-type",
	}

	synctest.Test(t, func(t *testing.T) {
		err = senderServer.SendMessage(leaderId, message)
		require.NoError(t, err)
		synctest.Wait()
	})
}

func TestCentral_HandleMessage_HandlesValidMessage(t *testing.T) {
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
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(1)

	centralConsensus := newPassiveCentral(leaderServer, &config)
	centralConsensus.RegisterListener(mockListener)

	bundle := types.Bundle{
		Transactions: []types.Transaction{{From: 1, To: 2, Value: 10}},
		Number:       123,
	}

	bundleMsg := BundleMessage{
		Bundle: bundle,
	}

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: bundleMsg,
	}

	synctest.Test(t, func(t *testing.T) {
		// Send the valid message - first time
		err = senderServer.SendMessage(leaderId, message)
		require.NoError(t, err)

		// Send the same message again to test duplicate handling - second time
		err = senderServer.SendMessage(leaderId, message)
		require.NoError(t, err)

		synctest.Wait()
	})
}

func TestCentral_Broadcast_HandlesNetworkSendError(t *testing.T) {
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

	config := Factory{
		EmitInterval: testInterval,
		Leader:       leader,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).
		MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	centralConsensus := config.NewActive(mockServer, mockSource)
	centralConsensus.RegisterListener(mockListener)

	// Give time for bundle to be created and for broadcast to be attempted
	// (which will fail)
	time.Sleep(2 * testInterval)
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
				bundle, ok := msg.Payload.(BundleMessage)
				require.True(t, ok, "unexpected message format")
				require.Equal(t, next, bundle.Bundle.Number, "unexpected bundle number")
				next++
			},
		).AnyTimes()

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateTransactions().MinTimes(1)

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

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateTransactions().MinTimes(1)
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		// Allow some emissions to occur. 2 * numEmissions because each receival
		// triggers another broadcast.
		server.EXPECT().GetPeers().Times(2 * numEmissions)

		centralConsensus := newActiveCentral(
			server,
			source,
			&Factory{EmitInterval: generic.DefaultEmitInterval},
		)
		time.Sleep(numEmissions * generic.DefaultEmitInterval)

		centralConsensus.Stop()
		server.EXPECT().GetPeers().Times(0)
		// Wait to ensure no further emissions occur.
		time.Sleep(2 * generic.DefaultEmitInterval)
	})
}

func TestCentral_Stop_StopsBundleReceivingAndProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
		server.EXPECT().RegisterMessageHandler(gomock.Any())
		consensus := newPassiveCentral(server, &Factory{generic.DefaultEmitInterval})

		// A gossip broadcast should trigger a server send, and also trigger
		// a [Central.addBundle] call which should trigger another broadcast (and server send).
		// Thus the 2 expected calls to GetPeers.
		server.EXPECT().GetPeers().Times(2)
		consensus.gossip.Broadcast(BundleMessage{})
		// Notification of local listeners is asynchronous, so wait.
		synctest.Wait()

		consensus.Stop()
		// After stopping the consensus instance, received bundles should not
		// enter the processing pipeline, meaning no further calls to GetPeers
		// by [Central.addBundle].
		server.EXPECT().GetPeers().Times(1)
		// Different bundle number to ensure it's not considered a duplicate by a gossip.
		consensus.gossip.Broadcast(BundleMessage{Bundle: types.Bundle{Number: 2}})
		synctest.Wait()
	})
}
