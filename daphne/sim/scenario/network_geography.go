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

package scenario

import (
	"github.com/0xsoniclabs/daphne/daphne/utils"
)

// NetworkGeography models the geographical distribution of nodes in a
// network, defining regions and the network latencies for intra-region and
// inter-region communication.
type NetworkGeography struct {
	numRegions int

	localSendLatency       utils.Distribution
	interRegionSendLatency utils.Distribution

	localDeliveryLatency       utils.Distribution
	interRegionDeliveryLatency utils.Distribution
}

// NewNetworkGeography creates a new NetworkGeography with the specified
// parameters. If any of the latency distributions are nil, they default to zero
// delay.
func NewNetworkGeography(
	numRegions int,
	localSendLatency utils.Distribution,
	interRegionSendLatency utils.Distribution,
	localDeliveryLatency utils.Distribution,
	interRegionDeliveryLatency utils.Distribution,
) NetworkGeography {
	if localDeliveryLatency == nil {
		localDeliveryLatency = utils.FixedDelay(0)
	}
	if interRegionDeliveryLatency == nil {
		interRegionDeliveryLatency = utils.FixedDelay(0)
	}
	if localSendLatency == nil {
		localSendLatency = utils.FixedDelay(0)
	}
	if interRegionSendLatency == nil {
		interRegionSendLatency = utils.FixedDelay(0)
	}
	return NetworkGeography{
		numRegions:                 numRegions,
		localSendLatency:           localSendLatency,
		interRegionSendLatency:     interRegionSendLatency,
		localDeliveryLatency:       localDeliveryLatency,
		interRegionDeliveryLatency: interRegionDeliveryLatency,
	}
}

// NewSimpleNetworkGeography creates a NetworkGeography with a single region
// and equal send/delivery latency distributions.
func NewSimpleNetworkGeography(sendDelay, deliveryDelay utils.Distribution) NetworkGeography {
	return NewNetworkGeography(
		1,
		sendDelay,
		sendDelay,
		deliveryDelay,
		deliveryDelay,
	)
}

func (ng NetworkGeography) GetNumRegions() int {
	return ng.numRegions
}

func (ng NetworkGeography) GetLocalSendLatency() utils.Distribution {
	return ng.localSendLatency
}

func (ng NetworkGeography) GetInterRegionSendLatency() utils.Distribution {
	return ng.interRegionSendLatency
}

func (ng NetworkGeography) GetLocalDeliveryLatency() utils.Distribution {
	return ng.localDeliveryLatency
}

func (ng NetworkGeography) GetInterRegionDeliveryLatency() utils.Distribution {
	return ng.interRegionDeliveryLatency
}
