package p2p

import (
	"sync"
	"time"
)

//go:generate mockgen -source network_delay_model.go \
// -destination=network_delay_model_mock.go -package=p2p

// LatencyModel defines how network delays are calculated between peers.
type LatencyModel interface {
	GetDelay(from, to PeerId, msg Message) time.Duration
}

// FixedDelayModel implements a latency model with a base delay and
// asymmetric per-connection custom delays.
type FixedDelayModel struct {
	baseDelay    time.Duration
	customDelays map[connectionKey]time.Duration

	// delaysMutex protects access to baseDelay and customDelays.
	delaysMutex sync.RWMutex
}

// NewFixedDelayModel creates a new fixed delay model with no initial delays.
func NewFixedDelayModel() *FixedDelayModel {
	return &FixedDelayModel{
		customDelays: make(map[connectionKey]time.Duration),
	}
}

type connectionKey struct {
	from PeerId
	to   PeerId
}

// SetBaseDelay sets a delay applied to all connections.
func (m *FixedDelayModel) SetBaseDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseDelay = delay
}

// SetConnectionDelay sets an additional delay for messages from one peer to
// another.
func (m *FixedDelayModel) SetConnectionDelay(from, to PeerId,
	delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customDelays[connectionKey{from: from, to: to}] = delay
}

// GetDelay returns the delay for a message from one peer to another.
func (m *FixedDelayModel) GetDelay(from, to PeerId, msg Message) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()

	delay := m.baseDelay
	if connDelay, exists :=
		m.customDelays[connectionKey{from: from, to: to}]; exists {
		// We presently set the connection delay instead of the base delay
		delay = connDelay
	}
	return delay
}
