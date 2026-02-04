package hotstuff

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

func TestHotstuff_MultipleHonestNodesExperienceConsistency(t *testing.T) {
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
			Delta:     250 * time.Millisecond,
			Tau:       7 * 250 * time.Millisecond,
			ViewLimit: numBundles + 1, // The +1 is for the fact that grandparents get committed, noting that genesis also gets committed.
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
		time.Sleep(factory.Tau)
	})
}

func TestHotstuff_InactiveNodeCannotDisruptHonestNodesConsistency(t *testing.T) {
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
			Delta: 250 * time.Millisecond,
			Tau:   7 * 250 * time.Millisecond,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		// Preallocate slice to avoid data race.
		bundles := make([][]types.Bundle, numNodes)
		hs := make([]consensus.Consensus, numNodes)

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
			hs[i] = factory.NewActive(servers[i],
				*committee,
				consensus.ValidatorId(i),
				src,
			)
			hs[i].RegisterListener(listeners[i])
		}

		wg.Wait()
		// Verify all nodes have the same bundles.
		reference := bundles[0]
		for i := range bundles {
			require.Equal(t, reference, bundles[i])
		}
		for i := range numNodes {
			hs[i].Stop()
		}
		// Wait for all goroutines to finish.
		time.Sleep(factory.Tau)
	})
}

func TestHotstuff_EquivocatorCannotDisruptHonestNodesConsistency(t *testing.T) {
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
			Delta:     250 * time.Millisecond,
			Tau:       7 * 250 * time.Millisecond,
			ViewLimit: numBundles + 1,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		bundles := make([][]types.Bundle, numNodes)
		hotstuff := make([]*Hotstuff, numNodes)

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
			hs := factory.NewActive(servers[i],
				*committee,
				consensus.ValidatorId(i),
				src,
			).(*Hotstuff)
			hs.RegisterListener(listeners[i])
			hotstuff[i] = hs
		}
		// Make node 3 equivocate by broadcasting fake votes.
		go func() {
			for {
				time.Sleep(100 * time.Millisecond)
				hotstuff[3].stateMutex.Lock()
				if hotstuff[3].stopped {
					hotstuff[3].stateMutex.Unlock()
					return
				}
				// Create a fake block with different transactions
				fakeBlock := Block{
					PrevHash: types.Hash{},
					View:     hotstuff[3].curView,
					Justify:  hotstuff[3].lockedQC,
					Payload: []types.Transaction{
						{From: 0, To: 0, Value: 0, Nonce: 0},
						{From: 0, To: 1, Value: 2, Nonce: 3},
					},
				}
				fakeMsg := Message{
					Signature: hotstuff[3].selfId,
					Type:      Vote,
					View:      hotstuff[3].curView,
					Contents: MessageVoteContents{
						BlockHash: fakeBlock.Hash(),
					},
				}
				go hotstuff[3].gossip.Broadcast(fakeMsg)
				hotstuff[3].stateMutex.Unlock()
			}
		}()

		wg.Wait()
		// Verify all honest nodes have the same bundles.
		reference := bundles[0]
		for i := range numNodes - 1 {
			require.Equal(t, reference, bundles[i])
		}
		// Wait for all goroutines to finish.
		time.Sleep(factory.Tau)
	})
}
