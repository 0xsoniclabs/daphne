package p2p

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/utils"
)

//go:generate mockgen -source network_delay_model.go -destination=network_delay_model_mock.go -package=p2p

// LatencyModel defines how network delays are calculated between peers.
// GetSendDelay represents the time until a message actually leaves the sender.
// GetDeliveryDelay represents the time until a message is delivered to the
// receiver.
type LatencyModel interface {
	GetSendDelay(from, to PeerId, msg Message) time.Duration
	GetDeliveryDelay(from, to PeerId, msg Message) time.Duration
}

// connectionKey uniquely identifies a directed connection between two peers.
type connectionKey struct {
	from PeerId
	to   PeerId
}

// DelayModel implements a latency model with a base delay distribution and
// asymmetric per-connection custom delay distributions for both send and delivery.
type DelayModel struct {
	sendDistribution     *utils.DelayModel[connectionKey]
	deliveryDistribution *utils.DelayModel[connectionKey]
}

// NewDelayModel creates a new delay model with no initial delays.
func NewDelayModel() *DelayModel {
	return &DelayModel{
		sendDistribution:     utils.NewDelayModel[connectionKey](),
		deliveryDistribution: utils.NewDelayModel[connectionKey](),
	}
}

func (m *DelayModel) SetBaseSendDistribution(delay utils.Distribution) *DelayModel {
	m.sendDistribution.ConfigureBase(delay)
	return m
}

func (m *DelayModel) SetConnectionSendDistribution(
	from, to PeerId,
	delay utils.Distribution,
) *DelayModel {
	m.sendDistribution.ConfigureCustom(connectionKey{from: from, to: to}, delay)
	return m
}

func (m *DelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.sendDistribution.GetDelay(connectionKey{from: from, to: to})
}

func (m *DelayModel) SetBaseDeliveryDistribution(delay utils.Distribution) *DelayModel {
	m.deliveryDistribution.ConfigureBase(delay)
	return m
}

func (m *DelayModel) SetConnectionDeliveryDistribution(
	from, to PeerId,
	delay utils.Distribution,
) *DelayModel {
	m.deliveryDistribution.ConfigureCustom(connectionKey{from: from, to: to}, delay)
	return m
}

func (m *DelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.deliveryDistribution.GetDelay(connectionKey{from: from, to: to})
}
