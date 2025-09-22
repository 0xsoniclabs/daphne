package p2p

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
)

func TestFixedDelayModel_SetBaseDeliveryDelay_CorrectlySetsBaseDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseDeliveryDelay(100 * time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseDeliveryDelay(250 * time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionDeliveryDelay_CorrectlySetsConnectionDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()
	model.SetBaseDeliveryDelay(100 * time.Millisecond)

	delay := model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionDeliveryDelay("peer1", "peer2", 150*time.Millisecond)
	delay = model.GetDeliveryDelay("peer1", "peer2", Message{})
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetDeliveryDelay("peer2", "peer1", Message{})
	require.Equal(100*time.Millisecond, delay)
}

func TestFixedDelayModel_SetBaseSendDelay_CorrectlySetsBaseSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseSendDelay(100 * time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseSendDelay(250 * time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionSendDelay_CorrectlySetsConnectionSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedDelayModel()
	model.SetBaseSendDelay(100 * time.Millisecond)

	delay := model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionSendDelay("peer1", "peer2", 150*time.Millisecond)
	delay = model.GetSendDelay("peer1", "peer2", Message{})
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetSendDelay("peer2", "peer1", Message{})
	require.Equal(100*time.Millisecond, delay)
}

func TestSampledDelayModel_SetSendDistribution_SamplesDelaysCorrectly(t *testing.T) {
	unit := time.Millisecond
	seed := int64(42)
	tests := map[string]struct {
		setDelay func(*SampledDelayModel)
	}{
		"base send distribution": {
			setDelay: func(model *SampledDelayModel) {
				model.SetBaseSendDistribution(utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
		"connection send distribution": {
			setDelay: func(model *SampledDelayModel) {
				model.SetConnectionSendDistribution("peer1", "peer2", utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewSampledDelayModel()

			initialDelay := model.GetSendDelay("peer1", "peer2", Message{})
			require.Equal(0*time.Millisecond, initialDelay)

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetSendDelay("peer1", "peer2", Message{})
				require.Greater(delays[i], 0*time.Millisecond)
			}

			allSame := true
			for _, delay := range delays {
				if delay != delays[0] {
					allSame = false
					break
				}
			}
			require.False(allSame, "Expected varying delays from log-normal distribution")
		})
	}
}

func TestSampledDelayModel_SetDeliveryDistribution_SamplesDelaysCorrectly(t *testing.T) {
	unit := time.Millisecond
	seed := int64(42)
	tests := map[string]struct {
		setDelay func(*SampledDelayModel)
	}{
		"base delivery distribution": {
			setDelay: func(model *SampledDelayModel) {
				model.SetBaseDeliveryDistribution(utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
		"connection delivery distribution": {
			setDelay: func(model *SampledDelayModel) {
				model.SetConnectionDeliveryDistribution("peer1", "peer2", utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewSampledDelayModel()

			initialDelay := model.GetDeliveryDelay("peer1", "peer2", Message{})
			require.Equal(0*time.Millisecond, initialDelay)

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetDeliveryDelay("peer1", "peer2", Message{})
				require.Greater(delays[i], 0*time.Millisecond)
			}

			allSame := true
			for _, delay := range delays {
				if delay != delays[0] {
					allSame = false
					break
				}
			}
			require.False(allSame, "Expected varying delays from log-normal distribution")
		})
	}
}

func TestSampledDelayModel_SetConnectionSendDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel()

	unit := time.Millisecond
	seed := int64(42)
	model.SetBaseSendDistribution(utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed))
	model.SetConnectionSendDistribution("peer1", "peer2", utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed))

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetSendDelay("peer1", "peer2", Message{})
		baseDelay := model.GetSendDelay("peer2", "peer1", Message{})
		require.Greater(customDelay, 0*unit)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}

func TestSampledDelayModel_SetConnectionDeliveryDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)
	model := NewSampledDelayModel()

	unit := time.Millisecond
	seed := int64(42)
	model.SetBaseDeliveryDistribution(utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed))
	model.SetConnectionDeliveryDistribution("peer1", "peer2", utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed))

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetDeliveryDelay("peer1", "peer2", Message{})
		baseDelay := model.GetDeliveryDelay("peer2", "peer1", Message{})
		require.Greater(customDelay, 0*unit)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}
