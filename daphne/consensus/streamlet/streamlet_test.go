package streamlet

import (
	"fmt"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStreamlet_Factory_ImplementsConsensusFactory(t *testing.T) {
	var _ consensus.Factory = &Factory{}
}

func TestStreamlet_NewActive_InstatiatesActiveStreamletAndRegistersListenersAndStartsEmittingBundles(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	leaderCreatorId := model.CreatorId(1)
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
		leaderCreatorId: 1,
	})
	require.NoError(t, err)

	config := Factory{
		EpochDuration: 1 * time.Second,
		Committee:     *committee,
		SelfId:        leaderCreatorId,
	}

	transactions := []types.Transaction{}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	streamletConsensus := config.NewActive(server, mockSource)
	streamletConsensus.RegisterListener(mockListener)

	// Sleep to be sure emitting has started.
	time.Sleep(2 * config.EpochDuration)
}

func TestStreamlet_NewPassive_InstantiatesPassiveStreamletAndRegistersListener(t *testing.T) {
	ctrl := gomock.NewController(t)

	network := p2p.NewNetwork()
	server, err := network.NewServer(p2p.PeerId("me"))
	someOtherServer, err := network.NewServer(p2p.PeerId("otherNode"))
	require.NoError(t, err)

	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
		model.CreatorId(123): 1, // some random id, not belonging to node1
	})
	require.NoError(t, err)

	config := Factory{
		Committee: *committee,
	}

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	streamletConsensus := config.NewPassive(server)
	streamletConsensus.RegisterListener(mockListener)
	someOtherServer.SendMessage(server.GetLocalId(), p2p.Message{
		Code:    p2p.MessageCode_StreamletConsensus_NewBundle,
		Payload: BundleMessage{},
	})

	// Sleep to make sure message has gone through.
	time.Sleep(100 * time.Millisecond)
}

func TestStreamlet_SingleActiveNodeChainsAndFinalizesBundles(t *testing.T) {
	ctrl := gomock.NewController(t)

	network := p2p.NewNetwork()
	server, err := network.NewServer(p2p.PeerId("leader"))
	require.NoError(t, err)

	leaderCreatorId := model.CreatorId(1)
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
		leaderCreatorId: 1,
	})
	require.NoError(t, err)

	config := Factory{
		EpochDuration: 1 * time.Second,
		Committee:     *committee,
		SelfId:        leaderCreatorId,
	}

	transactions := []types.Transaction{}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

	sc := config.NewActive(server, mockSource).(*Streamlet)

	// Set a small offset from epoch start, in order to be sure we are performing
	// checks after an epoch transition has been finished.
	time.Sleep(100 * time.Millisecond)

	// Check the number of blocks emitted and finalized, per epoch.
	expectedChainLength := []int{1, 2, 3, 4, 5}
	expectedFinalizedCount := []int{1, 1, 2, 3, 4}
	for epoch := range 5 {
		sc.stateMutex.Lock()

		chainLength := sc.chainLength(sc.hashToBundle[sc.longestNotarizedChains[0]])
		require.Equal(t,
			chainLength,
			expectedChainLength[epoch],
			fmt.Sprintf("in epoch %d chain length should be %d, is %d",
				epoch, expectedChainLength[epoch], chainLength),
		)
		require.Len(t, sc.finalizedBundles, expectedFinalizedCount[epoch])

		sc.stateMutex.Unlock()
		time.Sleep(1 * time.Second)
	}
}
