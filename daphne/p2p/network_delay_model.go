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

// --- FixedDelayModel ---

// FixedDelayModel implements a latency model with a base delay and
// asymmetric per-connection custom delays for both send and delivery.
type FixedDelayModel struct {
	sendDelay     utils.DelayModel[PeerId]
	deliveryDelay utils.DelayModel[PeerId]
}

// NewFixedDelayModel creates a new fixed delay model with no initial delays.
func NewFixedDelayModel() *FixedDelayModel {
	return &FixedDelayModel{
		sendDelay:     utils.NewFixedDelayModel[PeerId](),
		deliveryDelay: utils.NewFixedDelayModel[PeerId](),
	}
}

// SetBaseSendDelay sets a delay applied to all connections for sending messages
// (time before a message leaves the sender).
func (m *FixedDelayModel) SetBaseSendDelay(delay time.Duration) {
	m.sendDelay.ConfigureBaseDelay(delay)
}

// SetConnectionSendDelay sets a custom delay for sending messages from one
// peer to another, overriding the base send delay.
func (m *FixedDelayModel) SetConnectionSendDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.sendDelay.ConfigureCustomDelay(from, to, delay)
}

// GetSendDelay returns the send delay for a message from one peer to another.
func (m *FixedDelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.sendDelay.GetDelay(from, to)
}

// SetBaseDeliveryDelay sets a delay applied to all connections for message
// delivery.
func (m *FixedDelayModel) SetBaseDeliveryDelay(delay time.Duration) {
	m.deliveryDelay.ConfigureBaseDelay(delay)
}

// SetConnectionDeliveryDelay sets a custom delay for delivering messages from
// one peer to another, overriding the base delivery delay.
func (m *FixedDelayModel) SetConnectionDeliveryDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.deliveryDelay.ConfigureCustomDelay(from, to, delay)
}

// GetDeliveryDelay returns the delivery delay for a message from one peer to
// another.
func (m *FixedDelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.deliveryDelay.GetDelay(from, to)
}

// --- SampledDelayModel ---

// SampledDelayModel implements a latency model that samples delays from
// log-normal distributions, providing realistic network delay simulation
// with natural variation and occasional long-tail delays. The base send
// distribution is used for all connections that don't have custom
// distributions, while custom distributions can be set for specific
// connections asymmetrically. The same applies to delivery delays.
type SampledDelayModel struct {
	sendDistribution     *utils.SampledDelayModel[PeerId]
	deliveryDistribution *utils.SampledDelayModel[PeerId]

	// timeUnit defines the unit for sampled delays (e.g., time.Millisecond).
	timeUnit time.Duration
}

// NewSampledDelayModel creates a new sampled delay model.
// timeUnit specifies the unit for the sampled delays (e.g., time.Millisecond).
func NewSampledDelayModel(timeUnit time.Duration) *SampledDelayModel {
	return &SampledDelayModel{
		sendDistribution:     utils.NewSampledDelayModel[PeerId](timeUnit),
		deliveryDistribution: utils.NewSampledDelayModel[PeerId](timeUnit),
		timeUnit:             timeUnit,
	}
}

// SetBaseSendDistribution sets a log-normal distribution used for sampling
// send delays for all connections that don't have custom distributions.
func (m *SampledDelayModel) SetBaseSendDistribution(
	mu,
	sigma float64,
	seed *int64,
) {
	m.sendDistribution.SetBaseDistribution(mu, sigma, seed)
}

// SetConnectionSendDistribution sets a custom log-normal distribution for
// sampling send delays from one peer to another, overriding the base
// distribution.
func (m *SampledDelayModel) SetConnectionSendDistribution(
	from, to PeerId,
	mu, sigma float64,
	seed *int64,
) {
	m.sendDistribution.SetCustomDistribution(from, to, mu, sigma, seed)
}

// GetSendDelay returns a sampled send delay for a message from one peer to
// another.
func (m *SampledDelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.sendDistribution.GetDelay(from, to)
}

// SetBaseDeliveryDistribution sets a log-normal distribution used for sampling
// delivery delays for all connections that don't have custom distributions.
func (m *SampledDelayModel) SetBaseDeliveryDistribution(
	mu,
	sigma float64,
	seed *int64,
) {
	m.deliveryDistribution.SetBaseDistribution(mu, sigma, seed)
}

// SetConnectionDeliveryDistribution sets a custom log-normal distribution for
// sampling delivery delays from one peer to another, overriding the base
// distribution.
func (m *SampledDelayModel) SetConnectionDeliveryDistribution(
	from, to PeerId,
	mu, sigma float64,
	seed *int64,
) {
	m.deliveryDistribution.SetCustomDistribution(from, to, mu, sigma, seed)
}

// GetDeliveryDelay returns a sampled delivery delay for a message from one
// peer to another.
func (m *SampledDelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	return m.deliveryDistribution.GetDelay(from, to)
}
