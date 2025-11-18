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

		demo := &DemoScenario{
			NumValidators: 3,
			Topology:      mockTopology,
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}

func TestDemoScenario_Run_NilNetworkLatencyModel_DoesNotFail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker).AnyTimes()
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		demo := &DemoScenario{
			NumValidators:       3,
			NetworkLatencyModel: nil,
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
		mockLatencyModel := p2p.NewMockLatencyModel(ctrl)
		mockLatencyModel.EXPECT().GetSendDelay(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(5 * time.Millisecond).MinTimes(1)
		mockLatencyModel.EXPECT().GetDeliveryDelay(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(10 * time.Millisecond).MinTimes(1)

		demo := &DemoScenario{
			NumValidators:       3,
			NetworkLatencyModel: mockLatencyModel,
		}
		require.NoError(t, demo.Run(logger, tracker))
		time.Sleep(1 * time.Second)
	})
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
