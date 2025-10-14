package streamlet

import (
	"fmt"
	"testing"
	"testing/synctest"
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
	synctest.Test(t, func(t *testing.T) {
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

		// Make sure listener is not called, as no non-genesis block is finalized yet.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		consensus := config.NewActive(server, mockSource)
		consensus.RegisterListener(mockListener)
		defer consensus.Stop()

		// Sleep until after the first epoch transition, to be sure
		// a bundle has been emitted.
		time.Sleep(1 * config.EpochDuration)

		// Check that genesis block is finalized.
		sc := consensus.(*Streamlet)
		sc.stateMutex.Lock()
		_, exists := sc.finalizedBlocks[BlockMessage{}.Hash()]
		sc.stateMutex.Unlock()
		require.True(t, exists, "genesis block should be finalized")
		// Check that one bundle has been emitted.
		sc.stateMutex.Lock()
		require.Len(t, sc.hashToBlock, 2,
			"one bundle should be emitted, aside from genesis")
		sc.stateMutex.Unlock()
	})
}

func TestStreamlet_NewPassive_InstantiatesPassiveStreamletAndGenesisBlockFinalizedButListenersNotNotified(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
			model.CreatorId(123): 1, // some random id, not belonging to node1
		})
		require.NoError(t, err)

		config := Factory{
			Committee: *committee,
		}

		// Make sure listener is not called, even if genesis is finalized.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		consensus := config.NewPassive(server)
		consensus.RegisterListener(mockListener)
		defer consensus.Stop()

		// Check that genesis block is finalized.
		sc := consensus.(*Streamlet)
		sc.stateMutex.Lock()
		_, exists := sc.finalizedBlocks[BlockMessage{}.Hash()]
		sc.stateMutex.Unlock()
		require.True(t, exists, "genesis block should be finalized")
	})
}

func TestStreamlet_SingleActiveNodeChainsAndFinalizesBlocks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer sc.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		expectedBlockCount := []int{1, 2, 3, 4, 5}
		expectedFinalizedCount := []int{1, 1, 2, 3, 4}
		for epoch := range len(expectedBlockCount) {
			sc.stateMutex.Lock()

			chainLength := sc.longestNotarizedChainsLength
			require.Equal(t,
				chainLength,
				expectedBlockCount[epoch],
				fmt.Sprintf("in epoch %d chain length should be %d, is %d",
					epoch, expectedBlockCount[epoch], chainLength),
			)
			require.Len(t, sc.finalizedBlocks, expectedFinalizedCount[epoch],
				fmt.Sprintf("in epoch %d finalized count should be %d, is %d,",
					epoch, expectedFinalizedCount[epoch], len(sc.finalizedBlocks)),
			)

			sc.stateMutex.Unlock()
			time.Sleep(config.EpochDuration)
		}
	})
}

func TestStreamlet_SinglePassiveNodeChainsAndFinalizesBlocksWhenReceivingThemFromActiveNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		network := p2p.NewNetwork()

		// Create active node.
		activeServer, err := network.NewServer(p2p.PeerId("leader"))
		require.NoError(t, err)
		leaderCreatorId := model.CreatorId(1)
		committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)
		epochDuration := 1 * time.Second
		activeConfig := Factory{
			EpochDuration: epochDuration,
			Committee:     *committee,
			SelfId:        leaderCreatorId,
		}
		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)
		activeConsensus := activeConfig.NewActive(activeServer, mockSource)
		defer activeConsensus.Stop()

		// Create passive node.
		passiveServer, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)
		passiveConfig := Factory{
			EpochDuration: epochDuration,
			Committee:     *committee,
			SelfId:        model.CreatorId(2),
		}
		passiveConsensus := passiveConfig.NewPassive(passiveServer)
		defer passiveConsensus.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		expectedChainLength := []int{1, 2, 3, 4, 5}
		expectedFinalizedCount := []int{1, 1, 2, 3, 4}
		sc := passiveConsensus.(*Streamlet)
		for epoch := range len(expectedChainLength) {
			sc.stateMutex.Lock()

			chainLength := sc.longestNotarizedChainsLength
			require.Equal(t,
				chainLength,
				expectedChainLength[epoch],
				fmt.Sprintf("in epoch %d chain length should be %d, is %d",
					epoch, expectedChainLength[epoch], chainLength),
			)
			require.Len(t, sc.finalizedBlocks, expectedFinalizedCount[epoch],
				fmt.Sprintf("in epoch %d finalized count should be %d, is %d,",
					epoch, expectedFinalizedCount[epoch], len(sc.finalizedBlocks)),
			)

			sc.stateMutex.Unlock()
			time.Sleep(epochDuration)
		}
	})
}

func TestStreamlet_FinalizationNotifiesListenersProperly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		transactions := []types.Transaction{
			{From: 123, To: 456, Value: 10, Nonce: 0},
		}
		expectedBundle := types.Bundle{
			Number:       1,
			Transactions: transactions,
		}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

		sc := config.NewActive(server, mockSource).(*Streamlet)
		defer sc.Stop()

		mockListener := consensus.NewMockBundleListener(ctrl)
		// Expect to be called once, when a non-genesis block is finalized.
		mockListener.EXPECT().OnNewBundle(expectedBundle).Times(1)
		sc.RegisterListener(mockListener)

		// Sleep until after the first epoch transition, to be sure
		// a bundle has been emitted.
		time.Sleep(1 * config.EpochDuration)

		// Fake, manual finalization of the first block. This simplifies testing,
		// avoiding engaging with other parts of the logic.
		sc.finalizeBlock(sc.longestNotarizedChains[0])
	})
}
