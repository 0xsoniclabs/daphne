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

func (ns NetworkGeography) GetNumRegions() int {
	return ns.numRegions
}

func (ns NetworkGeography) GetLocalSendLatency() utils.Distribution {
	return ns.localSendLatency
}

func (ns NetworkGeography) GetInterRegionSendLatency() utils.Distribution {
	return ns.interRegionSendLatency
}

func (ns NetworkGeography) GetLocalDeliveryLatency() utils.Distribution {
	return ns.localDeliveryLatency
}

func (ns NetworkGeography) GetInterRegionDeliveryLatency() utils.Distribution {
	return ns.interRegionDeliveryLatency
}
