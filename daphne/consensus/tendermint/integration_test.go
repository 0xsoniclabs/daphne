package tendermint

import (
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTendermint_MultipleHonestNodesExperienceConsistency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		const numNodes = 2
		const numBundles = 1000
		stakeMap := make(map[consensus.ValidatorId]uint32)
		for i := range numNodes {
			stakeMap[consensus.ValidatorId(i)] = 1
		}
		committee, err := consensus.NewCommittee(stakeMap)
		require.NoError(t, err)

		latency := p2p.NewFixedDelayModel()
		latency.SetBaseSendDelay(0 * time.Millisecond)
		latency.SetBaseDeliveryDelay(200 * time.Millisecond)
		network := p2p.NewNetworkBuilder().WithLatency(latency).Build()

		factory := &Factory{
			Committee:   *committee,
			HeightLimit: numBundles,
		}
		servers := make([]p2p.Server, numNodes)
		for i := range numNodes {
			servers[i], err = network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i)))
			require.NoError(t, err)
		}
		listeners := make([]*consensus.MockBundleListener, numNodes)
		bundles := make([][]types.Bundle, numNodes)

		wg := sync.WaitGroup{}
		wg.Add(numNodes)
		for i := range numNodes {
			listeners[i] = consensus.NewMockBundleListener(ctrl)
			listeners[i].EXPECT().OnNewBundle(gomock.Any()).AnyTimes().Do(
				func(bundle types.Bundle) {
					// Preallocate slice to avoid data race
					bundles[i] = append(bundles[i], bundle)
					fmt.Printf("%d-%d\n", i, bundle.Number)
					if len(bundles[i]) == numBundles {
						wg.Done()
					}
				})
			bundles[i] = make([]types.Bundle, 0, numBundles)
			src := consensus.NewMockTransactionProvider(ctrl)
			src.EXPECT().GetCandidateTransactions().AnyTimes().Return([]types.Transaction{})
			factory.NewActive(servers[i],
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
	})
}
