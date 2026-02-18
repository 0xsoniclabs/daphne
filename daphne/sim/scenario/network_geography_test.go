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

package scenario

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
)

func TestNetworkStructure_NewNetworkStructure_SetsParametersCorrectly(t *testing.T) {
	require := require.New(t)
	localSendDist := utils.FixedDelay(5 * time.Millisecond)
	interRegionSendDist := utils.FixedDelay(15 * time.Millisecond)
	localDeliveryDist := utils.FixedDelay(7 * time.Millisecond)
	interRegionDeliveryDist := utils.FixedDelay(20 * time.Millisecond)

	geography := NewNetworkGeography(
		4,
		localSendDist,
		interRegionSendDist,
		localDeliveryDist,
		interRegionDeliveryDist,
	)

	require.Equal(4, geography.GetNumRegions())
	require.Equal(localSendDist, geography.GetLocalSendLatency())
	require.Equal(interRegionSendDist, geography.GetInterRegionSendLatency())
	require.Equal(localDeliveryDist, geography.GetLocalDeliveryLatency())
	require.Equal(interRegionDeliveryDist, geography.GetInterRegionDeliveryLatency())
}

func TestNetworkStructure_NewNetworkStructure_DefaultsNilArgumentsToZeroFixedDelay(t *testing.T) {
	require := require.New(t)
	geography := NewNetworkGeography(3, nil, nil, nil, nil)

	require.Zero(geography.GetInterRegionSendLatency())
	require.Zero(geography.GetLocalSendLatency())
	require.Zero(geography.GetInterRegionDeliveryLatency())
	require.Zero(geography.GetLocalDeliveryLatency())
}

func TestNetworkStructure_NewSimpleNetworkStructure_HasSingleRegionAndMimickedLatencies(t *testing.T) {
	require := require.New(t)
	dist1 := utils.FixedDelay(5 * time.Millisecond)
	dist2 := utils.FixedDelay(10 * time.Millisecond)
	geography := NewSimpleNetworkGeography(dist1, dist2)

	require.Equal(1, geography.GetNumRegions())
	require.Equal(dist1, geography.GetInterRegionSendLatency())
	require.Equal(dist1, geography.GetLocalSendLatency())
	require.Equal(dist2, geography.GetInterRegionDeliveryLatency())
	require.Equal(dist2, geography.GetLocalDeliveryLatency())
}
