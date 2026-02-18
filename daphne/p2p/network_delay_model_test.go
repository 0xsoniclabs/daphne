// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
)

func TestFixedDelayModel_SetBaseDeliveryDelay_CorrectlySetsBaseDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewDelayModel()

	delay := model.GetDeliveryDelay("peer1", "peer2", 12)
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseDeliveryDistribution(utils.FixedDelay(100 * time.Millisecond))
	delay = model.GetDeliveryDelay("peer1", "peer2", 12)
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseDeliveryDistribution(utils.FixedDelay(250 * time.Millisecond))
	delay = model.GetDeliveryDelay("peer1", "peer2", 12)
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionDeliveryDelay_CorrectlySetsConnectionDeliveryDelay(t *testing.T) {
	require := require.New(t)
	model := NewDelayModel()
	model.SetBaseDeliveryDistribution(utils.FixedDelay(100 * time.Millisecond))

	delay := model.GetDeliveryDelay("peer1", "peer2", 12)
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionDeliveryDistribution("peer1", "peer2", utils.FixedDelay(150*time.Millisecond))
	delay = model.GetDeliveryDelay("peer1", "peer2", 12)
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetDeliveryDelay("peer2", "peer1", 12)
	require.Equal(100*time.Millisecond, delay)
}

func TestFixedDelayModel_SetBaseSendDelay_CorrectlySetsBaseSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewDelayModel()

	delay := model.GetSendDelay("peer1", "peer2", 12)
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseSendDistribution(utils.FixedDelay(100 * time.Millisecond))
	delay = model.GetSendDelay("peer1", "peer2", 12)
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseSendDistribution(utils.FixedDelay(250 * time.Millisecond))
	delay = model.GetSendDelay("peer1", "peer2", 12)
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedDelayModel_SetConnectionSendDelay_CorrectlySetsConnectionSendDelay(t *testing.T) {
	require := require.New(t)
	model := NewDelayModel()
	model.SetBaseSendDistribution(utils.FixedDelay(100 * time.Millisecond))

	delay := model.GetSendDelay("peer1", "peer2", 12)
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionSendDistribution("peer1", "peer2", utils.FixedDelay(150*time.Millisecond))
	delay = model.GetSendDelay("peer1", "peer2", 12)
	require.Equal(150*time.Millisecond, delay)

	delay = model.GetSendDelay("peer2", "peer1", 12)
	require.Equal(100*time.Millisecond, delay)
}

func TestSampledDelayModel_SetSendDistribution_SamplesDelaysCorrectly(t *testing.T) {
	unit := time.Millisecond
	seed := int64(42)
	tests := map[string]struct {
		setDelay func(*DelayModel)
	}{
		"base send distribution": {
			setDelay: func(model *DelayModel) {
				model.SetBaseSendDistribution(utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
		"connection send distribution": {
			setDelay: func(model *DelayModel) {
				model.SetConnectionSendDistribution("peer1", "peer2", utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewDelayModel()

			initialDelay := model.GetSendDelay("peer1", "peer2", 12)
			require.Equal(0*time.Millisecond, initialDelay)

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetSendDelay("peer1", "peer2", 12)
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
		setDelay func(*DelayModel)
	}{
		"base delivery distribution": {
			setDelay: func(model *DelayModel) {
				model.SetBaseDeliveryDistribution(utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
		"connection delivery distribution": {
			setDelay: func(model *DelayModel) {
				model.SetConnectionDeliveryDistribution("peer1", "peer2", utils.NewLogNormalDistribution(1.5, 0.4, unit, &seed))
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			model := NewDelayModel()

			initialDelay := model.GetDeliveryDelay("peer1", "peer2", 12)
			require.Equal(0*time.Millisecond, initialDelay)

			testCase.setDelay(model)

			delays := make([]time.Duration, 10000)
			for i := range 10000 {
				delays[i] = model.GetDeliveryDelay("peer1", "peer2", 12)
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
	model := NewDelayModel()

	unit := time.Millisecond
	seed := int64(42)
	model.SetBaseSendDistribution(utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed))
	model.SetConnectionSendDistribution("peer1", "peer2", utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed))

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetSendDelay("peer1", "peer2", 12)
		baseDelay := model.GetSendDelay("peer2", "peer1", 12)
		require.Greater(customDelay, 0*unit)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}

func TestSampledDelayModel_SetConnectionDeliveryDistribution_OverridesBaseDistribution(t *testing.T) {
	require := require.New(t)
	model := NewDelayModel()

	unit := time.Millisecond
	seed := int64(42)
	model.SetBaseDeliveryDistribution(utils.NewLogNormalDistribution(1.0, 0.3, unit, &seed))
	model.SetConnectionDeliveryDistribution("peer1", "peer2", utils.NewLogNormalDistribution(3.0, 0.2, unit, &seed))

	// The delay is approximately exp(μ + σ * Z), where Z ~ Normal(0,1).
	// Hence, the higher the μ and σ, the higher the expected delay.
	for range 10000 {
		customDelay := model.GetDeliveryDelay("peer1", "peer2", 12)
		baseDelay := model.GetDeliveryDelay("peer2", "peer1", 12)
		require.Greater(customDelay, 0*unit)
		require.Greater(baseDelay, 0*unit)
		require.Greater(customDelay, baseDelay, "Expected custom delivery delays to be larger than base delays")
	}
}
