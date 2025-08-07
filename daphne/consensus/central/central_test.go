package central_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCentral_NetworkWithThreeNodes_CanProcessTransactions(t *testing.T) {
	require := require.New(t)

	leaderId := p2p.PeerId("leader")
	id1 := p2p.PeerId("node1")
	id2 := p2p.PeerId("node2")

	network := p2p.NewNetwork()

	genesis := map[types.Address]state.Account{
		1: {Balance: 100},
	}

	algorithm := central.Algorithm{
		EmitInterval: 100 * time.Millisecond,
	}

	_, err := node.NewValidator(leaderId, genesis, network, algorithm)
	require.NoError(err)
	node1, err := node.NewRpc(id1, genesis, network, algorithm)
	require.NoError(err)
	node2, err := node.NewRpc(id2, genesis, network, algorithm)
	require.NoError(err)

	tx := types.Transaction{From: 1, To: 2, Value: 10}

	rpc1 := node1.GetRpcService()
	require.NoError(rpc1.Send(tx))

	time.Sleep(300 * time.Millisecond) // Allow time for gossip propagation

	rpc2 := node2.GetRpcService()
	receipt, err := rpc2.GetReceipt(tx.Hash())
	require.NoError(err)
	require.NotNil(receipt)
	require.True(receipt.Success)
}

func TestCentral_NewActiveCentral_SetsEmitIntervalToDefaultIfNotSpecified(
	t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	algorithm := central.Algorithm{
		EmitInterval: 0, // Should use DefaultEmitInterval
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockPayloadSource(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).AnyTimes()

	centralConsensus := central.NewActiveCentral(server, mockSource, &algorithm)
	centralConsensus.RegisterListener(mockListener)

	time.Sleep(2 * central.DefaultEmitInterval)
	centralConsensus.Stop()
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

	algorithm := central.Algorithm{
		EmitInterval: 100 * time.Millisecond,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

	centralConsensus := central.NewPassiveCentral(server, &algorithm)
	centralConsensus.RegisterListener(mockListener)

	message := p2p.Message{
		Code:    p2p.MessageCode_CentralConsensus_NewBundle,
		Payload: "invalid-payload-type",
	}

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
	mockServer.EXPECT().SendMessage(failingPeer, gomock.Any()).
		Return(fmt.Errorf("network error")).AnyTimes()

	algorithm := central.Algorithm{
		EmitInterval: 100 * time.Millisecond,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockPayloadSource(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

	mockServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

	centralConsensus := central.NewActiveCentral(mockServer, mockSource,
		&algorithm)

	time.Sleep(150 * time.Millisecond)
	centralConsensus.Stop()
}
