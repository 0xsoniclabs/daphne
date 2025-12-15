package tendermint

import (
	"fmt"
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

func TestTendermint_MultipleHonestNodesExperienceConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 5
		const numBundles = 20
		stakeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes {
			stakeMap[consensus.ValidatorId(i)] = 1
		}
		committee, err := consensus.NewCommittee(stakeMap)
		require.NoError(t, err)

		latency := p2p.NewFixedDelayModel()
		latency.SetBaseSendDelay(10 * time.Millisecond)
		latency.SetBaseDeliveryDelay(200 * time.Millisecond)
		network := p2p.NewNetworkBuilder().WithLatency(latency).Build()

		factory := &Factory{
			HeightLimit: numBundles,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		// Preallocate slice to avoid data race.
		bundles := make([][]types.Bundle, numNodes)

		wg := sync.WaitGroup{}
		wg.Add(numNodes)
		for i := range numNodes {
			listeners[i] = consensus.NewMockBundleListener(ctrl)
			listeners[i].EXPECT().OnNewBundle(gomock.Any()).AnyTimes().Do(
				func(bundle types.Bundle) {
					bundles[i] = append(bundles[i], bundle)
					if len(bundles[i]) == numBundles {
						wg.Done()
					}
				})
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
			src := consensus.NewMockTransactionProvider(ctrl)
			src.EXPECT().GetCandidateLineup().AnyTimes().Return(lineup)
			factory.NewActive(servers[i],
				*committee,
				consensus.ValidatorId(i),
				src,
			).RegisterListener(listeners[i])
		}

		wg.Wait()
		// Verify all nodes have the same bundles.
		reference := bundles[0]
		for i := range bundles {
			require.Equal(t, reference, bundles[i])
		}
		// Wait for all goroutines to finish.
		time.Sleep(DefaultPhaseTimeout)
	})
}

func TestTendermint_InactiveNodeCannotDisruptHonestNodesConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 5
		const numBundles = 20
		stakeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes + numNodes/2 - 1 {
			stakeMap[consensus.ValidatorId(i)] = 1
		}
		committee, err := consensus.NewCommittee(stakeMap)
		require.NoError(t, err)

		latency := p2p.NewFixedDelayModel()
		latency.SetBaseSendDelay(10 * time.Millisecond)
		latency.SetBaseDeliveryDelay(200 * time.Millisecond)
		network := p2p.NewNetworkBuilder().WithLatency(latency).Build()

		factory := &Factory{
			HeightLimit: numBundles,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		// Preallocate slice to avoid data race.
		bundles := make([][]types.Bundle, numNodes)

		wg := sync.WaitGroup{}
		wg.Add(numNodes)
		for i := range numNodes {
			listeners[i] = consensus.NewMockBundleListener(ctrl)
			listeners[i].EXPECT().OnNewBundle(gomock.Any()).AnyTimes().Do(
				func(bundle types.Bundle) {

					bundles[i] = append(bundles[i], bundle)
					if len(bundles[i]) == numBundles {
						wg.Done()
					}
				})
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
			src := consensus.NewMockTransactionProvider(ctrl)
			src.EXPECT().GetCandidateLineup().AnyTimes().Return(lineup)
			factory.NewActive(servers[i],
				*committee,
				consensus.ValidatorId(i),
				src,
			).RegisterListener(listeners[i])
		}

		wg.Wait()
		// Verify all nodes have the same bundles.
		reference := bundles[0]
		for i := range bundles {
			require.Equal(t, reference, bundles[i])
		}
		// Wait for all goroutines to finish.
		time.Sleep(DefaultPhaseTimeout)
	})
}

func TestTendermint_EquivocatorCannotDisruptHonestNodesConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 4
		const numBundles = 20
		stakeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes {
			stakeMap[consensus.ValidatorId(i)] = 1
		}
		committee, err := consensus.NewCommittee(stakeMap)
		require.NoError(t, err)

		latency := p2p.NewFixedDelayModel()
		latency.SetBaseSendDelay(10 * time.Millisecond)
		latency.SetBaseDeliveryDelay(200 * time.Millisecond)
		network := p2p.NewNetworkBuilder().WithLatency(latency).Build()

		factory := &Factory{
			HeightLimit: numBundles,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		bundles := make([][]types.Bundle, numNodes)
		tendermint := make([]*Tendermint, numNodes)

		wg := sync.WaitGroup{}
		wg.Add(numNodes - 1)
		for i := range numNodes {
			listeners[i] = consensus.NewMockBundleListener(ctrl)
			listeners[i].EXPECT().OnNewBundle(gomock.Any()).AnyTimes().Do(
				func(bundle types.Bundle) {
					if i == 3 {
						// Equivocator node does not count towards the wait group.
						return
					}
					// Preallocate slice to avoid data race
					bundles[i] = append(bundles[i], bundle)
					if len(bundles[i]) == numBundles {
						wg.Done()
					}
				})
			bundles[i] = make([]types.Bundle, 0, numBundles)
			lineup := txpool.NewMockLineup(ctrl)
			lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
			src := consensus.NewMockTransactionProvider(ctrl)
			src.EXPECT().GetCandidateLineup().AnyTimes().Return(lineup)
			tm := factory.NewActive(servers[i],
				*committee,
				consensus.ValidatorId(i),
				src,
			).(*Tendermint)
			tm.RegisterListener(listeners[i])
			tendermint[i] = tm
		}
		// Make node 3 equivocate.
		go func() {
			for {
				time.Sleep(100 * time.Millisecond)
				tendermint[3].stateMutex.Lock()
				if tendermint[3].stopSignal == nil {
					return
				}
				fakeMsg := Message{
					Height:    tendermint[3].height,
					Round:     tendermint[3].round,
					Signature: 3,
					Phase:     Propose,
					Block: &Block{
						Transactions: []types.Transaction{
							{From: 0, To: 0, Value: 0, Nonce: 0},
							{From: 0, To: 1, Value: 2, Nonce: 3},
						},
						Number: uint32(747),
					}, // different block to cause equivocation
				}
				go tendermint[3].gossip.Broadcast(fakeMsg)
				tendermint[3].stateMutex.Unlock()
			}
		}()

		wg.Wait()
		// Verify all nodes have the same bundles.
		reference := bundles[0]
		for i := range numNodes - 1 {
			require.Equal(t, reference, bundles[i])
		}
		// Wait for all goroutines to finish.
		time.Sleep(DefaultPhaseTimeout)
	})
}

// Equivocating proposals should not be ignored by honest nodes.
// The reason for this is to prevent a situation where honest nodes take different
// proposals as the recognized proposal for that round, possibly leading to liveness issues.
func TestTendermint_EquivocatingProposalIsNotIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		committee, _ := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			0: 1,
			1: 3,
		})

		network := p2p.NewNetworkBuilder().Build()
		server, _ := network.NewServer(p2p.PeerId("0"))
		passive := Factory{}.NewPassive(server, *committee).(*Tendermint)
		fakeBlock := &Block{
			Transactions: []types.Transaction{
				{From: 67, To: 69, Value: 420, Nonce: 1312},
			},
			Number: 1,
		}
		realBlock := &Block{
			Transactions: []types.Transaction{},
			Number:       1,
		}
		proposal1 := Message{
			Signature:  0,
			Phase:      Propose,
			Block:      fakeBlock,
			Height:     0,
			Round:      0,
			PolkaRound: -1,
		}
		proposal2 := Message{
			Signature:  0,
			Phase:      Propose,
			Block:      realBlock,
			Height:     0,
			Round:      0,
			PolkaRound: -1,
		}
		prevote := Message{
			Signature: 1,
			Phase:     Prevote,
			BlockId:   realBlock.Id(),
			Height:    0,
			Round:     0,
		}
		passive.fakeHandleMessage(proposal1)
		passive.fakeHandleMessage(proposal2)
		passive.fakeHandleMessage(prevote)
		passive.stateMutex.Lock()
		require.NotNil(t, passive.lockedValue)
		require.Equal(t, realBlock.Id(), passive.lockedValue.Id())
		passive.stateMutex.Unlock()
		passive.Stop()
	})
}
