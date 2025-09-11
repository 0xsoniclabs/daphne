package p2p

import (
	"sync"
	"time"
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

// FixedDelayModel implements a latency model with a base delay and
// asymmetric per-connection custom delays for both send and delivery.
type FixedDelayModel struct {
	baseSendDelay        time.Duration
	customSendDelays     map[connectionKey]time.Duration
	baseDeliveryDelay    time.Duration
	customDeliveryDelays map[connectionKey]time.Duration

	// delaysMutex protects access to all delays.
	delaysMutex sync.RWMutex
}

// NewFixedDelayModel creates a new fixed delay model with no initial delays.
func NewFixedDelayModel() *FixedDelayModel {
	return &FixedDelayModel{
		customSendDelays:     make(map[connectionKey]time.Duration),
		customDeliveryDelays: make(map[connectionKey]time.Duration),
	}
}

type connectionKey struct {
	from PeerId
	to   PeerId
}

// SetBaseSendDelay sets a delay applied to all connections for sending messages
// (time before a message leaves the sender).
func (m *FixedDelayModel) SetBaseSendDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseSendDelay = delay
}

// GetSendDelay returns the send delay for a message from one peer to another.
func (m *FixedDelayModel) GetSendDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()

	delay := m.baseSendDelay
	if connDelay, exists := m.customSendDelays[connectionKey{from: from, to: to}]; exists {
		// Override the base send delay with the connection-specific send delay
		delay = connDelay
	}
	return delay
}

// SetConnectionSendDelay sets a custom delay for sending messages from one
// peer to another, overriding the base send delay.
func (m *FixedDelayModel) SetConnectionSendDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customSendDelays[connectionKey{from: from, to: to}] = delay
}

// SetBaseDeliveryDelay sets a delay applied to all connections for message
// delivery.
func (m *FixedDelayModel) SetBaseDeliveryDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseDeliveryDelay = delay
}

// SetConnectionDeliveryDelay sets a custom delay for delivering messages from
// one peer to another, overriding the base delivery delay.
func (m *FixedDelayModel) SetConnectionDeliveryDelay(
	from,
	to PeerId,
	delay time.Duration,
) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customDeliveryDelays[connectionKey{from: from, to: to}] = delay
}

// GetDeliveryDelay returns the delivery delay for a message from one peer to
// another.
func (m *FixedDelayModel) GetDeliveryDelay(
	from,
	to PeerId,
	msg Message,
) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()

	delay := m.baseDeliveryDelay
	if connDelay, exists := m.customDeliveryDelays[connectionKey{from: from, to: to}]; exists {
		// Override the base delivery delay with the connection-specific delivery
		// delay
		delay = connDelay
	}
	return delay
}
