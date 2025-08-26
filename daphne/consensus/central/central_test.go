package central_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
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

	const testInterval = central.DefaultEmitInterval

	config := central.Factory{
		EmitInterval: testInterval,
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

func TestCentral_NewPassive_InstantiatesPassiveCentralAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := central.Factory{}

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

	config := central.Factory{
		EmitInterval: 0,
	}

	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().
		Return([]types.Transaction{}).MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	centralConsensus := central.NewActiveCentral(server, mockSource, &config)
	centralConsensus.RegisterListener(mockListener)
	defer centralConsensus.Stop()

	time.Sleep(2 * central.DefaultEmitInterval)
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

	config := central.Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)

	centralConsensus := central.NewPassiveCentral(leaderServer, &config)
	centralConsensus.RegisterListener(mockListener)

	message := p2p.Message{
		Code: p2p.MessageCode_UnitTestProtocol_Ping,
	}

	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)
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

	config := central.Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)

	centralConsensus := central.NewPassiveCentral(leaderServer, &config)
	centralConsensus.RegisterListener(mockListener)

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: "invalid-payload-type",
	}

	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)
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

	config := central.Factory{}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(1)

	centralConsensus := central.NewPassiveCentral(leaderServer, &config)
	centralConsensus.RegisterListener(mockListener)

	bundle := types.Bundle{
		Transactions: []types.Transaction{{From: 1, To: 2, Value: 10}},
		Number:       123,
	}

	bundleMsg := central.BundleMessage{
		Bundle: bundle,
	}

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: bundleMsg,
	}

	// Send the valid message - first time
	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	// Send the same message again to test duplicate handling - second time
	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
}

func TestCentral_Broadcast_HandlesNetworkSendError(t *testing.T) {
	ctrl := gomock.NewController(t)

	peerId := p2p.PeerId("peer")
	mockServer := p2p.NewMockServer(ctrl)

	// Mock server returns a peer that will cause SendMessage to fail
	mockServer.EXPECT().GetLocalId().Return(p2p.PeerId("leader")).AnyTimes()
	mockServer.EXPECT().GetPeers().Return([]p2p.PeerId{peerId}).AnyTimes()
	mockServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

	// SendMessage will return an error to simulate network failure
	mockServer.EXPECT().SendMessage(peerId, gomock.Any()).
		Return(fmt.Errorf("network error")).AnyTimes()

	const testInterval = 100 * time.Millisecond

	config := central.Factory{
		EmitInterval: testInterval,
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
