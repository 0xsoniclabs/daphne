package scenario

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDemoScenario_Run_ZeroScenario_UsesDefaultValues(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker)
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(
			"Starting Demo Scenario",
			"numNodes", 3,
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
	tracker.EXPECT().With(gomock.Any()).Return(tracker)
	tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

	demo := &DemoScenario{
		nodeNameGenerator: func(int) string {
			return "server"
		},
	}

	require.ErrorContains(t, demo.Run(logger, tracker), "failed to create node")
}

func TestDemoScenario_Run_TransactionDuplicates_LogsWarnings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tracker := tracker.NewMockTracker(ctrl)
		tracker.EXPECT().With(gomock.Any()).Return(tracker)
		tracker.EXPECT().Track(gomock.Any(), gomock.Any()).AnyTimes()

		logger := NewMockLogger(ctrl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn("Failed to send transaction", "error", gomock.Any()).MinTimes(1)

		demo := &DemoScenario{
			transactionGenerator: func(int) types.Transaction {
				return types.Transaction{From: 0, To: 1, Nonce: 0}
			},
		}
		require.NoError(t, demo.Run(logger, tracker))
	})
}
