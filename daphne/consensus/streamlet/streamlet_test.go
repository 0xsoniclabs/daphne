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

func TestStreamlet_NewActiveReturnsStreamletInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	config := Factory{}
	consensus := config.NewActive(server, nil)
	sc := consensus.(*Streamlet)
	sc.Stop()
}

func TestStreamlet_NewPassiveReturnsStreamletInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	config := Factory{}
	consensus := config.NewPassive(server)
	sc := consensus.(*Streamlet)
	sc.Stop()
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

		const epochDuration = 1 * time.Second
		// Start 2 seconds from now.
		const timeUntilStart = time.Duration(2 * time.Second)

		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

		// Make sure listener is not called, as no non-genesis block is finalized yet.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		sc := newActiveStreamlet(
			server,
			mockSource,
			time.Now().Add(timeUntilStart),
			epochDuration,
			*committee,
			leaderCreatorId,
			nil,
			nil,
			nil,
		)
		sc.RegisterListener(mockListener)
		defer sc.Stop()

		// Sleep until after the first emission.
		time.Sleep(timeUntilStart + 1*epochDuration)
		sc.stateMutex.Lock()
		_, exists := sc.finalizedBlocks[BlockMessage{}.Hash()]
		sc.stateMutex.Unlock()
		require.True(t, exists, "genesis block should be finalized")
		// Check that one block message has been emitted (aside from genesis).
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

		// Make sure listener is not called, even if genesis is finalized.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		sc := newPassiveStreamlet(
			server,
			time.Time{},
			0,
			*committee,
			nil,
			nil,
		)
		defer sc.Stop()
		sc.RegisterListener(mockListener)
		defer sc.Stop()

		// Check that genesis block is finalized.
		sc.stateMutex.Lock()
		_, exists := sc.finalizedBlocks[BlockMessage{}.Hash()]
		sc.stateMutex.Unlock()
		require.True(t, exists, "genesis block should be finalized")
	})
}

func TestStreamlet_NewPassive_InvalidStartTimeGetsCorrected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
			model.CreatorId(1): 1,
		})
		require.NoError(t, err)

		const epochDuration time.Duration = 1 * time.Second
		now := time.Now()
		startTimes := []time.Time{
			now.Add(-epochDuration), {}, now.Add(epochDuration / 2), now.Add(epochDuration),
		}

		for _, startTime := range startTimes {
			sc := newPassiveStreamlet(
				server,
				startTime,
				epochDuration,
				*committee,
				nil,
				nil,
			)
			require.Equal(t, now.Add(epochDuration), sc.startTime,
				"start time should be corrected to the next epoch boundary")
			sc.Stop()
		}

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

		const epochDuration = 1 * time.Second
		// Start 2 seconds from now.
		const timeUntilStart = time.Duration(2 * time.Second)
		config := Factory{
			EpochDuration: epochDuration,
			Committee:     *committee,
			SelfId:        leaderCreatorId,
			StartTime:     time.Now().Add(timeUntilStart),
		}

		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

		sc := config.NewActive(server, mockSource).(*Streamlet)
		time.Sleep(timeUntilStart)
		defer sc.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		// Non-genesis blocks only start being finalized after there
		// are at least 3 blocks in the chain, due to the finalization rule.
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
			time.Sleep(epochDuration)
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
		const epochDuration = 1 * time.Second
		// Start 2 seconds from now.
		const timeUntilStart = time.Duration(2 * time.Second)
		activeConfig := Factory{
			EpochDuration: epochDuration,
			StartTime:     time.Now().Add(timeUntilStart),
			Committee:     *committee,
			SelfId:        leaderCreatorId,
		}
		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)
		activeConsensus := activeConfig.NewActive(activeServer, mockSource)
		time.Sleep(timeUntilStart)
		defer activeConsensus.Stop()

		// Create passive node.
		passiveServer, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)
		passiveConsensus := newPassiveStreamlet(
			passiveServer,
			time.Now().Add(epochDuration),
			epochDuration,
			*committee,
			nil,
			nil,
		)
		defer passiveConsensus.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		expectedChainLength := []int{1, 2, 3, 4, 5}
		expectedFinalizedCount := []int{1, 1, 2, 3, 4}
		for epoch := range len(expectedChainLength) {
			passiveConsensus.stateMutex.Lock()

			chainLength := passiveConsensus.longestNotarizedChainsLength
			require.Equal(t,
				chainLength,
				expectedChainLength[epoch],
				fmt.Sprintf("in epoch %d chain length should be %d, is %d",
					epoch, expectedChainLength[epoch], chainLength),
			)
			require.Len(t,
				passiveConsensus.finalizedBlocks, expectedFinalizedCount[epoch],
				fmt.Sprintf("in epoch %d finalized count should be %d, is %d,",
					epoch, expectedFinalizedCount[epoch], len(passiveConsensus.finalizedBlocks)),
			)

			passiveConsensus.stateMutex.Unlock()
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
		transactions := []types.Transaction{
			{From: 123, To: 456, Value: 10, Nonce: 0},
		}
		expectedBundle := types.Bundle{
			Number:       1,
			Transactions: transactions,
		}

		sc := newPassiveStreamlet(
			server,
			time.Time{},
			time.Duration(0),
			*committee,
			nil,
			nil,
		)
		defer sc.Stop()

		mockListener := consensus.NewMockBundleListener(ctrl)
		// Expect to be called once, when a non-genesis block is finalized.
		mockListener.EXPECT().OnNewBundle(expectedBundle).Times(1)
		sc.RegisterListener(mockListener)

		// Fake, manual finalization of some block. This simplifies testing,
		// avoiding engaging with other parts of the logic.
		sc.stateMutex.Lock()
		bm := BlockMessage{
			LastBlockHash: BlockMessage{}.Hash(),
			Transactions:  transactions,
		}
		sc.addBlock(bm)
		sc.finalizeBlock(bm.Hash())
		sc.stateMutex.Unlock()
	})
}

func TestStreamlet_BlocksNeverGetNotarizedOrFinalizedWithoutQuorum(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("leader"))
		require.NoError(t, err)

		leaderCreatorId := model.CreatorId(1)
		committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
			leaderCreatorId: 1,
			// This member does not exist in the network, preventing quorum
			// from being reached.
			model.CreatorId(2): 1,
		})
		require.NoError(t, err)
		const epochDuration = 1 * time.Second
		// Start 2 seconds from now.
		const timeUntilStart = time.Duration(2 * time.Second)
		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

		sc := newActiveStreamlet(
			server,
			mockSource,
			time.Now().Add(timeUntilStart),
			epochDuration,
			*committee,
			leaderCreatorId,
			nil,
			nil,
			nil,
		)
		mockListener := consensus.NewMockBundleListener(ctrl)
		// Expect to never be called, as no block can be finalized
		// without quorum.
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)
		sc.RegisterListener(mockListener)
		time.Sleep(timeUntilStart)
		defer sc.Stop()

		// Wait for a few epochs, so that the blocks would have been notarized
		// and finalized, had quorum been possible.
		time.Sleep(5 * epochDuration)

		sc.stateMutex.Lock()
		chainLength := sc.longestNotarizedChainsLength
		require.Equal(t, 1, chainLength,
			"chain length should be 1, is %d", chainLength)
		require.Len(t, sc.finalizedBlocks, 1,
			"finalized count should be 1, is %d", len(sc.finalizedBlocks))
		sc.stateMutex.Unlock()
	})
}
