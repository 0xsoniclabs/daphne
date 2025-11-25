package state

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
)

func TestFixedProcessingDelayModel_SetTransactionDelay_CorrectlySetsTransactionDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()

	txBase := types.Transaction{From: 2, To: 1, Value: 50, Nonce: 1}
	txCustom := types.Transaction{From: 1, To: 2, Value: 100, Nonce: 1}

	require.Equal(0*time.Millisecond, model.GetTransactionDelay(txBase))
	require.Equal(0*time.Millisecond, model.GetTransactionDelay(txCustom))

	model.SetBaseTransactionDelay(100 * time.Millisecond)
	require.Equal(100*time.Millisecond, model.GetTransactionDelay(txBase))
	require.Equal(100*time.Millisecond, model.GetTransactionDelay(txCustom))

	model.SetBaseTransactionDelay(250 * time.Millisecond)
	require.Equal(250*time.Millisecond, model.GetTransactionDelay(txBase))
	require.Equal(250*time.Millisecond, model.GetTransactionDelay(txCustom))

	model.SetConnectionTransactionDelay(1, 2, 300*time.Millisecond)
	require.Equal(250*time.Millisecond, model.GetTransactionDelay(txBase))
	require.Equal(300*time.Millisecond, model.GetTransactionDelay(txCustom))
}

func TestFixedProcessingDelayModel_SetBlockFinalizationDelay_CorrectlySetsBlockFinalizationDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()

	require.Equal(0*time.Millisecond, model.GetBlockFinalizationDelay(0, nil))
	require.Equal(0*time.Millisecond, model.GetBlockFinalizationDelay(22, nil))

	model.SetBaseBlockFinalizationDelay(100 * time.Millisecond)
	require.Equal(100*time.Millisecond, model.GetBlockFinalizationDelay(0, nil))
	require.Equal(100*time.Millisecond, model.GetBlockFinalizationDelay(22, nil))

	model.SetBaseBlockFinalizationDelay(150 * time.Millisecond)
	require.Equal(150*time.Millisecond, model.GetBlockFinalizationDelay(0, nil))
	require.Equal(150*time.Millisecond, model.GetBlockFinalizationDelay(22, nil))

	model.SetCustomBlockFinalizationDelay(22, 300*time.Millisecond)
	require.Equal(300*time.Millisecond, model.GetBlockFinalizationDelay(22, nil))
	require.Equal(150*time.Millisecond, model.GetBlockFinalizationDelay(0, nil))
}

func TestSampledProcessingDelayModel_SetTransactionDistribution_SamplesDelaysCorrectly(t *testing.T) {
	unit := time.Millisecond
	seed := int64(42)

	txBase := types.Transaction{From: 2, To: 1, Value: 10, Nonce: 1}
	txCustom := types.Transaction{From: 1, To: 2, Value: 10, Nonce: 1}

	tests := map[string]struct {
		setDelay func(*SampledProcessingDelayModel)
	}{
		"base transaction distribution": {
			setDelay: func(model *SampledProcessingDelayModel) {
				model.SetBaseTransactionDistribution(
					utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed),
				)
			},
		},
		"connection transaction distribution": {
			setDelay: func(model *SampledProcessingDelayModel) {
				model.SetConnectionTransactionDistribution(
					1, 2, utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed),
				)
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewSampledProcessingDelayModel()

			initialDelaytxBase := model.GetTransactionDelay(txBase)
			require.Equal(0*unit, initialDelaytxBase)
			initialDelaytxCustom := model.GetTransactionDelay(txCustom)
			require.Equal(0*unit, initialDelaytxCustom)

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetTransactionDelay(txCustom)
				require.Greater(delays[i], 0*unit)
				txBaseDelay := model.GetTransactionDelay(txBase)
				if testName == "base transaction distribution" {
					require.Greater(txBaseDelay, 0*unit)
				} else {
					require.Equal(txBaseDelay, 0*unit)
				}
			}

			allSame := true
			for _, d := range delays {
				if d != delays[0] {
					allSame = false
					break
				}
			}
			require.False(allSame, "Expected varying transaction delays from log-normal distribution")
		})
	}
}

func TestSampledProcessingDelayModel_SetBlockFinalizationDistribution_SamplesDelaysCorrectly(t *testing.T) {
	unit := time.Millisecond
	seed := int64(42)

	blockBase := uint32(20)
	blockCustom := uint32(10)

	tests := map[string]struct {
		setDelay func(*SampledProcessingDelayModel)
	}{
		"base finalization distribution": {
			setDelay: func(model *SampledProcessingDelayModel) {
				model.SetBaseBlockFinalizationDistribution(
					utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed),
				)
			},
		},
		"custom finalization distribution": {
			setDelay: func(model *SampledProcessingDelayModel) {
				model.SetCustomBlockFinalizationDistribution(
					10, utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed),
				)
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewSampledProcessingDelayModel()

			require.Equal(0*unit, model.GetBlockFinalizationDelay(blockBase, nil))
			require.Equal(0*unit, model.GetBlockFinalizationDelay(blockCustom, nil))

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetBlockFinalizationDelay(blockCustom, nil)
				require.Greater(delays[i], 0*unit)

				baseDelay := model.GetBlockFinalizationDelay(blockBase, nil)
				if testName == "base finalization distribution" {
					require.Greater(baseDelay, 0*unit)
				} else {
					require.Equal(baseDelay, 0*unit)
				}
			}

			allSame := true
			for _, d := range delays {
				if d != delays[0] {
					allSame = false
					break
				}
			}
			require.False(allSame, "Expected varying finalization delays from log-normal distribution")
		})
	}
}

func TestSampledProcessingDelayModel_SetConnectionTransactionDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	seed := int64(42)
	model := NewSampledProcessingDelayModel()
	model.SetBaseTransactionDistribution(
		utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed),
	)
	model.SetConnectionTransactionDistribution(
		1, 2, utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed),
	)

	txBase := types.Transaction{From: 2, To: 1, Value: 10, Nonce: 1}
	txCustom := types.Transaction{From: 1, To: 2, Value: 10, Nonce: 1}

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		baseDelay := model.GetTransactionDelay(txBase)
		customDelay := model.GetTransactionDelay(txCustom)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom transaction delay to be larger than base delay")
	}
}

func TestSampledProcessingDelayModel_SetCustomBlockFinalizationDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	seed := int64(42)
	model := NewSampledProcessingDelayModel()
	model.SetBaseBlockFinalizationDistribution(
		utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed),
	)
	model.SetCustomBlockFinalizationDistribution(
		10, utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed),
	)

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		baseDelay := model.GetBlockFinalizationDelay(20, nil)
		customDelay := model.GetBlockFinalizationDelay(10, nil)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom finalization delay to be larger than base delay")
	}
}

func TestSampledProcessingDelayModel_GetBaseTransactionDistribution_ReturnsSetDistribution(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	seed := int64(42)
	model := NewSampledProcessingDelayModel()

	require.Nil(model.GetBaseTransactionDistribution())

	dist := utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed)
	model.SetBaseTransactionDistribution(dist)

	retrieved := model.GetBaseTransactionDistribution()
	require.NotNil(retrieved)
	require.Equal(dist, retrieved)
}

func TestSampledProcessingDelayModel_GetBaseBlockFinalizationDistribution_ReturnsSetDistribution(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	seed := int64(42)
	model := NewSampledProcessingDelayModel()

	require.Nil(model.GetBaseBlockFinalizationDistribution())

	dist := utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed)
	model.SetBaseBlockFinalizationDistribution(dist)

	retrieved := model.GetBaseBlockFinalizationDistribution()
	require.NotNil(retrieved)
	require.Equal(dist, retrieved)
}
