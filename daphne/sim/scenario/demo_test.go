package scenario

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDemoScenario_Run_ZeroScenario_UsesDefaultValues(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(
			"Starting Demo Scenario",
			"numValidators", 3,
			"numRpcNodes", 1,
			"numObservers", 0,
			"txPerSecond", 100,
			"duration", 5*time.Second,
			"consensus", gomock.Any(),
		)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		demo := &DemoScenario{}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_NetworkCreationIssue_FailsScenario(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	tracker := tracker.NewMockTracker(ctrl)
	tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
	tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

	for _, prefix := range []string{"V", "R", "O"} {
		t.Run(prefix, func(t *testing.T) {
			demo := &DemoScenario{
				nodeNameGenerator: func(pre string, index int) string {
					if pre == prefix {
						return "server" // Duplicated name to trigger error
					}
					return fmt.Sprintf("%s%d", pre, index)
				},
				NumValidators: 2,
				NumRpcNodes:   2,
				NumObservers:  2,
			}

			require.ErrorContains(t, demo.Run(logger, tracker), "failed to create node")
		})
	}

}

func TestDemoScenario_Run_TransactionDuplicates_LogsWarnings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(
			"Failed to send transaction",
			"tx_counter", gomock.Any(),
			"error", gomock.Any(),
		).MinTimes(1)

		demo := &DemoScenario{
			transactionGenerator: func(int) types.Transaction {
				return types.Transaction{From: 0, To: 1, Nonce: 0}
			},
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_NilTopology_DoesNotFail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		demo := &DemoScenario{
			NumValidators: 3,
			Topology:      nil, // Explicitly nil to test default behavior
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_WithProvidedTopology_UsesTopology(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		// Test with a mock topology to verify it's actually being used
		mockTopology := p2p.NewMockNetworkTopology(ctrl)
		mockTopology.EXPECT().ShouldConnect(gomock.Any(), gomock.Any()).
			Return(true).MinTimes(1)

		mockFactory := p2p.NewMockTopologyFactory(ctrl)
		mockFactory.EXPECT().Create(gomock.Any()).Return(mockTopology)
		mockFactory.EXPECT().String().Return("mock-topology").AnyTimes()

		demo := &DemoScenario{
			NumValidators: 3,
			Topology:      mockFactory,
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_UnsetNetworkLatencyModel_DoesNotFail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		demo := &DemoScenario{
			NumValidators: 3,
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_WithMockNetworkLatencyModel_LatencyModelIsUsed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		// Test with a mock latency model to verify it's actually being used
		sendDist := utils.NewMockDistribution(ctrl)
		sendDist.EXPECT().SampleDuration().Return(5 * time.Millisecond).MinTimes(1)

		deliveryDist := utils.NewMockDistribution(ctrl)
		deliveryDist.EXPECT().SampleDuration().Return(10 * time.Millisecond).MinTimes(1)

		demo := &DemoScenario{
			NumValidators:    3,
			NetworkGeography: NewSimpleNetworkGeography(sendDist, deliveryDist),
		}
		require.NoError(t, demo.Run(logger, tracker))
		time.Sleep(1 * time.Second)
	})
}

func TestDemoScenario_getNetworkLatencyModel_AppliesMultiRegionNetworkGeography(t *testing.T) {
	require := require.New(t)

	localSendDist := utils.FixedDelay(1 * time.Millisecond)
	localDeliveryDist := utils.FixedDelay(2 * time.Millisecond)
	remoteSendDist := utils.FixedDelay(3 * time.Millisecond)
	remoteDeliveryDist := utils.FixedDelay(4 * time.Millisecond)

	peers := []p2p.PeerId{"A", "B", "C", "D", "E", "F", "G", "H"}
	regions := make([][]p2p.PeerId, 3)
	regions[0] = []p2p.PeerId{"A", "D", "G"}
	regions[1] = []p2p.PeerId{"B", "E", "H"}
	regions[2] = []p2p.PeerId{"C", "F"}

	networkDelay := getNetworkLatencyModel(peers, NewNetworkGeography(
		3,
		localSendDist,
		remoteSendDist,
		localDeliveryDist,
		remoteDeliveryDist,
	))

	for regionId, regionPeers := range regions {
		for otherRegionId, otherRegionPeers := range regions {
			for _, peer := range regionPeers {
				for _, otherPeer := range otherRegionPeers {
					sendDelay := networkDelay.GetSendDelay(peer, otherPeer, nil)
					deliveryDelay := networkDelay.GetDeliveryDelay(peer, otherPeer, nil)

					if regionId == otherRegionId {
						require.EqualValues(localSendDist, sendDelay, "peers %s and %s in same region", peer, otherPeer)
						require.EqualValues(localDeliveryDist, deliveryDelay, "peers %s and %s in same region", peer, otherPeer)
					} else {
						require.EqualValues(remoteSendDist, sendDelay, "peers %s and %s in different regions", peer, otherPeer)
						require.EqualValues(remoteDeliveryDist, deliveryDelay, "peers %s and %s in different regions", peer, otherPeer)
					}
				}
			}
		}
	}
}

func TestDemoScenario_Run_NilStateProcessingDelayModel_DoesNotFail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		demo := &DemoScenario{
			NumValidators:             3,
			StateProcessingDelayModel: nil,
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_WithMockStateProcessingDelayModel_DelayModelIsUsed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		// Test with a mock state processing delay model to verify it's actually being used
		mockDelayModel := state.NewMockProcessingDelayModel(ctrl)
		mockDelayModel.EXPECT().GetTransactionDelay(gomock.Any()).
			Return(2 * time.Millisecond).MinTimes(1)
		mockDelayModel.EXPECT().GetBlockFinalizationDelay(gomock.Any(), gomock.Any()).
			Return(5 * time.Millisecond).MinTimes(1)

		demo := &DemoScenario{
			NumValidators:             3,
			StateProcessingDelayModel: mockDelayModel,
		}
		require.NoError(t, demo.Run(logger, tracker))
		time.Sleep(1 * time.Second)
	})
}

func Test_singleUserTransactionGenerator_ProducesConsecutiveTransactionsFromAccount0(t *testing.T) {
	for i := range 10 {
		tx := singleUserTransactionGenerator(i)
		require.EqualValues(t, 0, tx.From)
		require.EqualValues(t, i, tx.Nonce)
	}
}

func Test_roundRobbingTransactionGenerator_ProducesTransactionsFromMultipleAccounts(t *testing.T) {
	numSenders := 3
	for i := range 10 {
		tx := roundRobbingTransactionGenerator(numSenders, i)
		expectedSender := types.Address(i % numSenders)
		expectedNonce := uint64(i / numSenders)
		require.EqualValues(t, expectedSender, tx.From)
		require.EqualValues(t, expectedNonce, tx.Nonce)
	}
}

func Test_defaultTransactionGenerator_CreatesASequenceOfProcessableTransactions(t *testing.T) {
	nonces := map[types.Address]types.Nonce{}
	for i := range 1000 {
		tx := defaultTransactionGenerator(i)
		expectedNonce := nonces[tx.From]
		require.EqualValues(t, expectedNonce, tx.Nonce)
		nonces[tx.From] = expectedNonce + 1
	}

	// Account 0 should have sent half of the transactions
	require.EqualValues(t, 500, nonces[0])

	// The rest is evenly distributed among 99 other accounts
	for addr := types.Address(1); addr < 100; addr++ {
		require.GreaterOrEqual(t, int(nonces[addr]), 5)
		require.LessOrEqual(t, int(nonces[addr]), 6)
	}
}
