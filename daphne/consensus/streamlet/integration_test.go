package streamlet

import (
	"fmt"
	"slices"
	"sync"
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

func TestStreamlet_MultipleHonestActiveNodesExperienceConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 20
		const numEpochs = 30
		const epochDuration = 1 * time.Second

		committeeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes {
			committeeMap[consensus.ValidatorId(i+1)] = 1
		}
		network := p2p.NewNetwork()
		scList := make([]*Streamlet, numNodes)
		listenerList := make([]*accumulatorListener, numNodes)
		for i := range numNodes {
			server, err := network.NewServer(p2p.PeerId(fmt.Sprintf("node%d", i+1)))
			require.NoError(t, err)

			creatorId := consensus.ValidatorId(i + 1)
			committee, err := consensus.NewCommittee(committeeMap)
			require.NoError(t, err)

			transactions := []types.Transaction{}
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return(transactions).AnyTimes()
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(lineup).MinTimes(1)

			scList[i] = newActiveStreamlet(
				server,
				mockSource,
				epochDuration,
				*committee,
				creatorId,
				nil,
			)
			// Ensure cleanup.
			defer scList[i].Stop()
			// Register a listener to accumulate bundles.
			listenerList[i] = &accumulatorListener{}
			scList[i].RegisterListener(listenerList[i])
		}

		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(numEpochs * epochDuration)

		// Check that all nodes have the same finalized blocks.
		for i := range numNodes - 1 {
			require.NotEmpty(t, listenerList[i].getBundles())
			require.Equal(t, listenerList[i].getBundles(), listenerList[i+1].getBundles())
		}
	})
}

func TestStreamlet_MultipleHonestNodesEmitUniformlyWithDefaultLeaderSelection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 20
		const numEpochs = 20
		const epochDuration = 1 * time.Second

		committeeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes {
			committeeMap[consensus.ValidatorId(i+1)] = 1
		}
		network := p2p.NewNetwork()
		for i := range numNodes {
			server, err := network.NewServer(p2p.PeerId(fmt.Sprintf("node%d", i+1)))
			require.NoError(t, err)

			creatorId := consensus.ValidatorId(i + 1)
			committee, err := consensus.NewCommittee(committeeMap)
			require.NoError(t, err)
			config := Factory{
				EpochDuration: epochDuration,
			}
			transactions := []types.Transaction{}
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return(transactions).AnyTimes()
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(lineup).Times(1)

			consensus := config.NewActive(server, *committee, creatorId, mockSource)
			defer consensus.Stop()
		}

		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(numEpochs * epochDuration)
	})
}

func TestStreamlet_InactiveNodeCannotDisruptHonestNodesConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		committeeMap := map[consensus.ValidatorId]uint32{
			consensus.ValidatorId(1): 1,
			consensus.ValidatorId(2): 1,
			consensus.ValidatorId(3): 1,
			consensus.ValidatorId(4): 1, // 4 is inactive.
		}
		const epochDuration = 1 * time.Second
		const numEpochs = 20
		network := p2p.NewNetwork()
		nodes := make([]*Streamlet, 4)
		honestListeners := make([]*accumulatorListener, 4)
		for i := range 4 {
			server, err := network.NewServer(p2p.PeerId(fmt.Sprintf("node%d", i+1)))
			require.NoError(t, err)

			creatorId := consensus.ValidatorId(i + 1)
			committee, err := consensus.NewCommittee(committeeMap)
			require.NoError(t, err)
			if i == 3 {
				creatorId = consensus.ValidatorId(100) // effectively inactive
			}
			transactions := []types.Transaction{}
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return(transactions).AnyTimes()
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(lineup).AnyTimes()

			nodes[i] = newActiveStreamlet(
				server,
				mockSource,
				epochDuration,
				*committee,
				creatorId,
				nil,
			)
			// Ensure cleanup.
			defer nodes[i].Stop()
			// Register a listener to accumulate bundles.
			honestListeners[i] = &accumulatorListener{}
			nodes[i].RegisterListener(honestListeners[i])
		}
		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(numEpochs * epochDuration)

		// Check that all honest nodes have the same finalized blocks.
		for i := range 2 {
			require.Equal(t, honestListeners[i].getBundles(), honestListeners[i+1].getBundles())
		}
	})
}

func TestStreamlet_EquivocatingLeaderCannotDisruptHonestNodesConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		committeeMap := map[consensus.ValidatorId]uint32{
			consensus.ValidatorId(1): 1,
			consensus.ValidatorId(2): 1,
			consensus.ValidatorId(3): 1,
			consensus.ValidatorId(4): 1, // 4 is equivocating leader.
		}
		const epochDuration = 1 * time.Second
		const numEpochs = 20
		network := p2p.NewNetwork()
		nodes := make([]*Streamlet, 4)
		honestListeners := make([]*accumulatorListener, 4)
		for i := range 4 {
			server, err := network.NewServer(p2p.PeerId(fmt.Sprintf("node%d", i+1)))
			require.NoError(t, err)

			creatorId := consensus.ValidatorId(i + 1)
			committee, err := consensus.NewCommittee(committeeMap)
			require.NoError(t, err)
			var emitProcedure func(s *Streamlet,
				source consensus.TransactionProvider) = nil
			// Make node 4 equivocate when it is leader.
			if i == 3 {
				emitProcedure = func(s *Streamlet,
					source consensus.TransactionProvider) {
					// Create two different blocks and broadcast both.
					if chooseLeader(s.getEpoch(), s.committee) == s.selfId {
						blockMessage1 := s.createProposalMessage(source)
						s.channel.Broadcast(blockMessage1)

						blockMessage2 := blockMessage1
						blockMessage2.Transactions = slices.Clone(blockMessage1.Transactions)
						blockMessage2.Transactions = []types.Transaction{
							{From: 123, To: 456, Value: 10, Nonce: 0},
						} // make it different
						s.channel.Broadcast(blockMessage2)
					}
				}
			}
			transactions := []types.Transaction{}
					lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).AnyTimes()
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(lineup).AnyTimes()

			nodes[i] = newActiveStreamlet(
				server,
				mockSource,
				epochDuration,
				*committee,
				creatorId,
				emitProcedure,
			)
			defer nodes[i].Stop()
			// Register a listener to accumulate bundles.
			honestListeners[i] = &accumulatorListener{}
			nodes[i].RegisterListener(honestListeners[i])
		}
		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(numEpochs * epochDuration)

		// Check that all honest nodes have the same finalized blocks.
		for i := range 2 {
			require.Equal(t, honestListeners[i].getBundles(), honestListeners[i+1].getBundles())
		}
	})
}

type accumulatorListener struct {
	bundles []types.Bundle
	mutex   sync.Mutex
}

func (al *accumulatorListener) getBundles() []types.Bundle {
	al.mutex.Lock()
	defer al.mutex.Unlock()
	return slices.Clone(al.bundles)
}

func (al *accumulatorListener) OnNewBundle(bundle types.Bundle) {
	al.mutex.Lock()
	defer al.mutex.Unlock()
	al.bundles = append(al.bundles, bundle)
}
