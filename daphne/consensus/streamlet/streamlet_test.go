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

package streamlet

import (
	"fmt"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStreamlet_Factory_ImplementsConsensusFactory(t *testing.T) {
	var _ consensus.Factory = &Factory{}
}

func TestStreamlet_Factory_String_ReturnsSummary(t *testing.T) {
	config := Factory{}
	require.Equal(t, "streamlet-1000ms", config.String())

	config = Factory{EpochDuration: 500 * time.Millisecond}
	require.Equal(t, "streamlet-500ms", config.String())
}

func TestStreamlet_NewActiveReturnsStreamletInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	config := Factory{}
	consensus := config.NewActive(server, consensus.Committee{}, 0, nil)
	sc := consensus.(*Streamlet)
	sc.Stop()
}

func TestStreamlet_NewPassiveReturnsStreamletInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	config := Factory{}
	consensus := config.NewPassive(server, consensus.Committee{})
	sc := consensus.(*Streamlet)
	sc.Stop()
}

func TestStreamlet_NewActive_InstantiatesActiveStreamletAndRegistersListenersAndStartsEmittingBundles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leaderId := p2p.PeerId("leader")
		network := p2p.NewNetwork()
		server, err := network.NewServer(leaderId)
		require.NoError(t, err)

		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)

		const epochDuration = 1 * time.Second

		transactions := []types.Transaction{}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		// Make sure listener is not called, as no non-genesis block is finalized yet.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		sc := newActiveStreamlet(
			server,
			mockSource,
			epochDuration,
			*committee,
			leaderCreatorId,
			nil,
		)
		sc.RegisterListener(mockListener)
		defer sc.Stop()

		// Sleep until after the first emission.
		time.Sleep(1 * epochDuration)
		sc.stateMutex.Lock()
		exists := sc.finalizedBlocks.Contains(BlockMessage{}.Hash())
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

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			consensus.ValidatorId(123): 1, // some random id, not belonging to node1
		})
		require.NoError(t, err)

		// Make sure listener is not called, even if genesis is finalized.
		// The reason is that the genesis block is finalized before any listener
		// is registered, so the listener should not be notified about it.
		mockListener := consensus.NewMockBundleListener(ctrl)
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)

		sc := newPassiveStreamlet(
			server,
			0,
			*committee,
		)
		defer sc.Stop()
		sc.RegisterListener(mockListener)

		// Check that genesis block is finalized.
		sc.stateMutex.Lock()
		exists := sc.finalizedBlocks.Contains(BlockMessage{}.Hash())
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

		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)

		const epochDuration = 1 * time.Second
		config := Factory{
			EpochDuration: epochDuration,
		}

		transactions := []types.Transaction{}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		sc := config.NewActive(server, *committee, leaderCreatorId, mockSource).(*Streamlet)
		defer sc.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		// Non-genesis blocks only start being finalized after there
		// are at least 3 blocks in the chain, due to the finalization rule.
		expectedBlockCount := []int{1, 2, 3, 4, 5}
		expectedFinalizedCount := []int{1, 1, 2, 3, 4}
		for epoch := range len(expectedBlockCount) {
			sc.stateMutex.Lock()
			chainLength := sc.longestNotarizedChainsLength
			numFinalizedBlocks := sc.finalizedBlocks.Size()
			sc.stateMutex.Unlock()

			require.Equal(t,
				chainLength,
				expectedBlockCount[epoch],
				fmt.Sprintf("in epoch %d chain length should be %d, is %d",
					epoch, expectedBlockCount[epoch], chainLength),
			)
			require.Equal(t, expectedFinalizedCount[epoch], numFinalizedBlocks,
				fmt.Sprintf("in epoch %d finalized count should be %d, is %d,",
					epoch, expectedFinalizedCount[epoch], numFinalizedBlocks),
			)

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
		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)
		const epochDuration = 1 * time.Second
		// Start 2 seconds from now.
		activeConfig := Factory{
			EpochDuration: epochDuration,
		}
		transactions := []types.Transaction{}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)
		activeConsensus := activeConfig.NewActive(activeServer, *committee, leaderCreatorId, mockSource)
		defer activeConsensus.Stop()

		// Create passive node.
		passiveServer, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)
		passiveConsensus := newPassiveStreamlet(
			passiveServer,
			epochDuration,
			*committee,
		)
		defer passiveConsensus.Stop()

		// Check the number of blocks emitted and finalized, per epoch.
		expectedChainLength := []int{1, 2, 3, 4, 5}
		expectedFinalizedCount := []int{1, 1, 2, 3, 4}
		for epoch := range len(expectedChainLength) {
			passiveConsensus.stateMutex.Lock()
			chainLength := passiveConsensus.longestNotarizedChainsLength
			numFinalizedBlocks := passiveConsensus.finalizedBlocks.Size()
			passiveConsensus.stateMutex.Unlock()

			require.Equal(t,
				chainLength,
				expectedChainLength[epoch],
				fmt.Sprintf("in epoch %d chain length should be %d, is %d",
					epoch, expectedChainLength[epoch], chainLength),
			)
			require.Equal(t,
				expectedFinalizedCount[epoch], numFinalizedBlocks,
				fmt.Sprintf("in epoch %d finalized count should be %d, is %d,",
					epoch, expectedFinalizedCount[epoch], numFinalizedBlocks),
			)

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
		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
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
			time.Duration(0),
			*committee,
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

		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
			// This member does not exist in the network, preventing quorum
			// from being reached.
			consensus.ValidatorId(2): 1,
		})
		require.NoError(t, err)
		const epochDuration = 1 * time.Second
		transactions := []types.Transaction{}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		sc := newActiveStreamlet(
			server,
			mockSource,
			epochDuration,
			*committee,
			leaderCreatorId,
			nil,
		)
		mockListener := consensus.NewMockBundleListener(ctrl)
		// Expect to never be called, as no block can be finalized
		// without quorum.
		mockListener.EXPECT().OnNewBundle(gomock.Any()).Times(0)
		sc.RegisterListener(mockListener)
		defer sc.Stop()

		// Wait for a few epochs, so that the blocks would have been notarized
		// and finalized, had quorum been possible.
		time.Sleep(5 * epochDuration)

		sc.stateMutex.Lock()
		finalizedBlocksCount := sc.finalizedBlocks.Size()
		chainLength := sc.longestNotarizedChainsLength
		sc.stateMutex.Unlock()
		require.Equal(t, 1, chainLength,
			"chain length should be 1, is %d", chainLength)
		require.Equal(t, finalizedBlocksCount, 1,
			"finalized count should be 1, is %d", finalizedBlocksCount)
	})
}

func TestStreamlet_Stop_StopsBundleEmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		const emissionCount = 5
		// 2 for: emission + voting.
		server.EXPECT().GetPeers().Times(2 * emissionCount)
		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)
		const epochDuration = 1 * time.Second
		transactions := []types.Transaction{}
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)

		sc := newActiveStreamlet(
			server,
			mockSource,
			epochDuration,
			*committee,
			leaderCreatorId,
			nil,
		)
		// Wait for a few epochs, so emissionCount messages are emitted.
		time.Sleep(emissionCount * epochDuration)
		sc.Stop()
		// Sleep a bit more to make sure no more emissions happen.
		time.Sleep(emissionCount * epochDuration)
	})
}

func TestStreamlet_Stop_StopsReceivingAndHandling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			consensus.ValidatorId(1): 1,
		})
		require.NoError(t, err)
		const epochDuration = 1 * time.Second
		sc := newPassiveStreamlet(
			server,
			epochDuration,
			*committee,
		)

		firstBlock := BlockMessage{
			LastBlockHash: BlockMessage{}.Hash(),
			Transactions:  nil,
		}
		server.EXPECT().GetPeers().Times(1)
		sc.stateMutex.Lock()
		sc.channel.Broadcast(firstBlock)
		sc.stateMutex.Unlock()
		synctest.Wait()

		sc.Stop()

		// Just the call itself, no receivers on gossip.
		server.EXPECT().GetPeers().Times(1)
		sc.stateMutex.Lock()
		secondBlock := BlockMessage{
			LastBlockHash: firstBlock.Hash(),
			Transactions:  nil,
		}
		sc.channel.Broadcast(secondBlock)
		sc.stateMutex.Unlock()
		synctest.Wait()
	})
}

func TestBlockMessage_MessageSize(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		transactions := []types.Transaction{
			{From: 1, To: 2, Value: 10, Nonce: 0},
			{From: 3, To: 4, Value: 20, Nonce: 1},
		}
		sizes := make([]uint32, len(transactions))
		for i, tx := range transactions {
			sizes[i] = tx.MessageSize()
		}
		bm := BlockMessage{
			LastBlockHash: types.Sha256([]byte("previous block")),
			Transactions:  transactions,
		}

		expectedSize := uint32(reflect.TypeFor[BlockMessage]().Size()) +
			sizes[0] + sizes[1]
		actualSize := bm.MessageSize()
		require.Equal(t, expectedSize, actualSize,
			"BlockMessage MessageSize should return the correct size")
	})
}
