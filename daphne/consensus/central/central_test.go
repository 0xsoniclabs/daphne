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

func TestCentral_NewActive_InstantiatesActiveCentralAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).AnyTimes()

	centralConsensus := config.NewActive(server, mockSource)
	centralConsensus.RegisterListener(mockListener)
}

func TestCentral_NewPassive_InstantiatesPassiveCentralAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).AnyTimes()

	centralConsensus := config.NewPassive(server)
	centralConsensus.RegisterListener(mockListener)
}

func TestCentral_NewActiveCentral_SetsEmitIntervalToDefaultIfNotSpecified(
	t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 0, // Should use DefaultEmitInterval
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).AnyTimes()

	centralConsensus := central.NewActiveCentral(server, mockSource, &config)
	centralConsensus.RegisterListener(mockListener)

	time.Sleep(2 * central.DefaultEmitInterval)
	centralConsensus.Stop()
}

func TestCentral_HandleMessage_HandlesInvalidMessageCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	senderId := p2p.PeerId("sender")

	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)
	senderServer, err := network.NewServer(senderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

	centralConsensus := central.NewPassiveCentral(server, &config)
	centralConsensus.RegisterListener(mockListener)

	message := p2p.Message{
		Code: p2p.MessageCode_UnitTestProtocol_Ping,
	}

	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestCentral_HandleMessage_HandlesInvalidBundlePayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	senderId := p2p.PeerId("sender")

	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)
	senderServer, err := network.NewServer(senderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

	centralConsensus := central.NewPassiveCentral(server, &config)
	centralConsensus.RegisterListener(mockListener)

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: "invalid-payload-type",
	}

	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestCentral_HandleMessage_HandlesValidMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	senderId := p2p.PeerId("sender")

	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)
	senderServer, err := network.NewServer(senderId)
	require.NoError(t, err)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	// Expect OnNewBundle to be called twice since we send the same message twice
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(2)

	centralConsensus := central.NewPassiveCentral(server, &config)
	centralConsensus.RegisterListener(mockListener)

	bundle := types.Bundle{
		Transactions: []types.Transaction{{From: 1, To: 2, Value: 10}},
	}

	bundleMsg := central.BundleMessage{
		Number: 123,
		Bundle: bundle,
	}

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: bundleMsg,
	}

	// Send the valid message - first time
	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	// Allow time for message processing
	time.Sleep(150 * time.Millisecond)

	// Send the same message again to test duplicate handling - second time
	err = senderServer.SendMessage(leaderId, message)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestCentral_Broadcast_HandlesNetworkSendError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")

	mockServer := p2p.NewMockServer(ctrl)
	mockServer.EXPECT().GetLocalId().Return(leaderId).AnyTimes()

	failingPeer := p2p.PeerId("failing-peer")
	mockServer.EXPECT().GetPeers().Return([]p2p.PeerId{failingPeer}).AnyTimes()
	mockServer.EXPECT().SendMessage(leaderId, gomock.Any()).
		Return(fmt.Errorf("network error")).Times(1)
	mockServer.EXPECT().SendMessage(failingPeer, gomock.Any()).
		Return(fmt.Errorf("network error")).Times(1)

	config := central.Factory{
		EmitInterval: 100 * time.Millisecond,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

	mockServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

	centralConsensus := central.NewActiveCentral(mockServer, mockSource,
		&config)

	time.Sleep(150 * time.Millisecond)
	centralConsensus.Stop()
}
