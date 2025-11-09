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

// --- FixedDelayModel ---

// FixedDelayModel implements a latency model with a base delay and
// asymmetric per-connection custom delays for both send and delivery.
type FixedDelayModel struct {
	sendDelay     *utils.DelayModel[connectionKey, time.Duration]
	deliveryDelay *utils.DelayModel[connectionKey, time.Duration]
}

// NewFixedDelayModel creates a new fixed delay model with no initial delays.
func NewFixedDelayModel() *FixedDelayModel {
	return &FixedDelayModel{
		sendDelay:     utils.NewFixedDelayModel[connectionKey](),
		deliveryDelay: utils.NewFixedDelayModel[connectionKey](),
	}
}

// SetBaseSendDelay sets a delay applied to all connections for sending messages
// (time before a message leaves the sender).
func (m *FixedDelayModel) SetBaseSendDelay(delay time.Duration) {
	m.sendDelay.ConfigureBase(delay)
}

// SetConnectionSendDelay sets a custom delay for sending messages from one
// peer to another, overriding the base send delay.
func (m *FixedDelayModel) SetConnectionSendDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.sendDelay.ConfigureCustom(connectionKey{from: from, to: to}, delay)
}

// GetSendDelay returns the send delay for a message from one peer to another.
func (m *FixedDelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.sendDelay.GetDelay(connectionKey{from: from, to: to})
}

// SetBaseDeliveryDelay sets a delay applied to all connections for message
// delivery.
func (m *FixedDelayModel) SetBaseDeliveryDelay(delay time.Duration) {
	m.deliveryDelay.ConfigureBase(delay)
}

// SetConnectionDeliveryDelay sets a custom delay for delivering messages from
// one peer to another, overriding the base delivery delay.
func (m *FixedDelayModel) SetConnectionDeliveryDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.deliveryDelay.ConfigureCustom(connectionKey{from: from, to: to}, delay)
}

// GetDeliveryDelay returns the delivery delay for a message from one peer to
// another.
func (m *FixedDelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.deliveryDelay.GetDelay(connectionKey{from: from, to: to})
}

// --- SampledDelayModel ---

// SampledDelayModel implements a latency model that samples delays from
// log-normal distributions, providing realistic network delay simulation
// with natural variation and occasional long-tail delays. The base send
// distribution is used for all connections that don't have custom
// distributions, while custom distributions can be set for specific
// connections asymmetrically. The same applies to delivery delays.
type SampledDelayModel struct {
	sendDistribution     *utils.DelayModel[connectionKey, utils.Distribution]
	deliveryDistribution *utils.DelayModel[connectionKey, utils.Distribution]
}

// NewSampledDelayModel creates a new sampled delay model.
// timeUnit specifies the unit for the sampled delays (e.g., time.Millisecond).
func NewSampledDelayModel() *SampledDelayModel {
	return &SampledDelayModel{
		sendDistribution:     utils.NewSampledDelayModel[connectionKey](),
		deliveryDistribution: utils.NewSampledDelayModel[connectionKey](),
	}
}

// SetBaseSendDistribution sets a log-normal distribution used for sampling
// send delays for all connections that don't have custom distributions.
func (m *SampledDelayModel) SetBaseSendDistribution(dist utils.Distribution) {
	m.sendDistribution.ConfigureBase(dist)
}

// GetBaseSendDistribution returns the base send distribution, or nil if not
// configured.
func (m *SampledDelayModel) GetBaseSendDistribution() utils.Distribution {
	return m.sendDistribution.GetBase()
}

// SetConnectionSendDistribution sets a custom log-normal distribution for
// sampling send delays from one peer to another, overriding the base
// distribution.
func (m *SampledDelayModel) SetConnectionSendDistribution(
	from, to PeerId,
	dist utils.Distribution,
) {
	m.sendDistribution.ConfigureCustom(connectionKey{from: from, to: to}, dist)
}

// GetSendDelay returns a sampled send delay for a message from one peer to
// another.
func (m *SampledDelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.sendDistribution.GetDelay(connectionKey{from: from, to: to})
}

// SetBaseDeliveryDistribution sets a log-normal distribution used for sampling
// delivery delays for all connections that don't have custom distributions.
func (m *SampledDelayModel) SetBaseDeliveryDistribution(dist utils.Distribution) {
	m.deliveryDistribution.ConfigureBase(dist)
}

// GetBaseDeliveryDistribution returns the base delivery distribution, or nil
// if not configured.
func (m *SampledDelayModel) GetBaseDeliveryDistribution() utils.Distribution {
	return m.deliveryDistribution.GetBase()
}

// SetConnectionDeliveryDistribution sets a custom log-normal distribution for
// sampling delivery delays from one peer to another, overriding the base
// distribution.
func (m *SampledDelayModel) SetConnectionDeliveryDistribution(
	from, to PeerId,
	dist utils.Distribution,
) {
	m.deliveryDistribution.ConfigureCustom(connectionKey{from: from, to: to}, dist)
}

// GetDeliveryDelay returns a sampled delivery delay for a message from one
// peer to another.
func (m *SampledDelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.deliveryDistribution.GetDelay(connectionKey{from: from, to: to})
}
