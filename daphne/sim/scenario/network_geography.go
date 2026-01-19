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
