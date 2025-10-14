package streamlet

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

func TestStreamlet_MultipleHonestActiveNodesExperienceConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 20
		const numEpochs = 30
		const epochDuration = 1 * time.Second
		const timeUntilStart = time.Duration(2 * time.Second)

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
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

			scList[i] = newActiveStreamlet(
				server,
				mockSource,
				time.Now().Add(timeUntilStart),
				epochDuration,
				*committee,
				creatorId,
				nil,
				nil,
				nil,
			)
			// Ensure cleanup.
			defer scList[i].Stop()
			// Register a listener to accumulate bundles.
			listenerList[i] = &accumulatorListener{}
			scList[i].RegisterListener(listenerList[i])
		}

		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(timeUntilStart + numEpochs*epochDuration)

		// Check that all nodes have the same finalized blocks.
		for i := range numNodes - 1 {
			require.NotEmpty(t, listenerList[i].bundles)
			require.Equal(t, listenerList[i].bundles, listenerList[i+1].bundles)
		}
	})
}

func TestStreamlet_MultipleHonestNodesEmitUniformlyWithDefaultLeaderSelection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 20
		const numEpochs = 20
		const epochDuration = 1 * time.Second
		const timeUntilStart = time.Duration(2 * time.Second)

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
				StartTime:     time.Now().Add(timeUntilStart),
				Committee:     *committee,
				SelfId:        creatorId,
			}
			transactions := []types.Transaction{}
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(transactions).Times(1)

			consensus := config.NewActive(server, mockSource)
			defer consensus.Stop()
		}

		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(timeUntilStart + numEpochs*epochDuration)
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
		const timeUntilStart = time.Duration(2 * time.Second)
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
			startTime := time.Now().Add(timeUntilStart)
			if i == 3 {
				startTime = time.Now().Add(100 * time.Hour) // effectively inactive
			}
			transactions := []types.Transaction{}
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

			nodes[i] = newActiveStreamlet(
				server,
				mockSource,
				startTime,
				epochDuration,
				*committee,
				creatorId,
				nil,
				nil,
				nil,
			)
			// Ensure cleanup.
			defer nodes[i].Stop()
			// Register a listener to accumulate bundles.
			honestListeners[i] = &accumulatorListener{}
			nodes[i].RegisterListener(honestListeners[i])
		}
		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(timeUntilStart + numEpochs*epochDuration)

		// Check that all honest nodes have the same finalized blocks.
		for i := range 2 {
			require.Equal(t, honestListeners[i].bundles, honestListeners[i+1].bundles)
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
		const timeUntilStart = time.Duration(2 * time.Second)
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
				source generic.EmissionPayloadSource[BlockMessage]) = nil
			// Make node 4 equivocate when it is leader.
			if i == 3 {
				emitProcedure = func(s *Streamlet,
					source generic.EmissionPayloadSource[BlockMessage]) {
					// Create two different blocks and broadcast both.
					if s.chooseLeaderProcedure(s.getEpoch(), s.committee) == s.selfId {
						blockMessage1 := source.GetEmissionPayload()
						s.gossip.Broadcast(blockMessage1)

						blockMessage2 := source.GetEmissionPayload()
						blockMessage2.Transactions = []types.Transaction{
							{From: 123, To: 456, Value: 10, Nonce: 0},
						} // make it different
						s.gossip.Broadcast(blockMessage2)
					}
				}
			}
			transactions := []types.Transaction{}
			mockSource := consensus.NewMockTransactionProvider(ctrl)
			mockSource.EXPECT().GetCandidateTransactions().Return(transactions).AnyTimes()

			nodes[i] = newActiveStreamlet(
				server,
				mockSource,
				time.Now().Add(timeUntilStart),
				epochDuration,
				*committee,
				creatorId,
				nil,
				emitProcedure,
				nil,
			)
			defer nodes[i].Stop()
			// Register a listener to accumulate bundles.
			honestListeners[i] = &accumulatorListener{}
			nodes[i].RegisterListener(honestListeners[i])
		}
		// Wait for a number of epochs, to let nodes emit and finalize blocks.
		time.Sleep(timeUntilStart + numEpochs*epochDuration)

		// Check that all honest nodes have the same finalized blocks.
		for i := range 2 {
			nodes[i].stateMutex.Lock()
			nodes[i+1].stateMutex.Lock()
			require.Equal(t, honestListeners[i].bundles, honestListeners[i+1].bundles)
			nodes[i].stateMutex.Unlock()
			nodes[i+1].stateMutex.Unlock()
		}
	})
}

type accumulatorListener struct {
	bundles []types.Bundle
}

func (al *accumulatorListener) OnNewBundle(bundle types.Bundle) {
	al.bundles = append(al.bundles, bundle)
}
