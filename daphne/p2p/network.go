package p2p

import (
	"fmt"
	"sync"
	"time"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers   map[PeerId]peer
	tasks   sync.WaitGroup
	latency LatencyModel

	// stateMutex protects access to the peers map and the latency model, and other
	// Network state.
	stateMutex sync.RWMutex
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return NewNetworkWithLatency(nil)
}

// NewNetworkWithLatency creates a new P2P network with a custom latency model.
func NewNetworkWithLatency(model LatencyModel) *Network {
	return &Network{
		peers:   make(map[PeerId]peer),
		latency: model,
	}
}

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

type connectionKey struct {
	from PeerId
	to   PeerId
}

// SetGlobalDelay sets a delay applied to all connections.
func (n *Network) SetGlobalDelay(delay time.Duration) {
	n.stateMutex.Lock()
	defer n.stateMutex.Unlock()

	// If no latency model exists, create a FixedDelayModel
	if n.latency == nil {
		n.latency = NewFixedDelayModel()
	}

	// If the latency model is a FixedDelayModel, use its SetBaseDelay method
	if fdm, ok := n.latency.(*FixedDelayModel); ok {
		fdm.SetBaseDelay(delay)
	}
}

// SetConnectionDelay sets a delay for messages from one peer to another.
func (n *Network) SetConnectionDelay(from, to PeerId, delay time.Duration) {
	n.stateMutex.Lock()
	defer n.stateMutex.Unlock()

	// If no latency model exists, create a FixedDelayModel
	if n.latency == nil {
		n.latency = NewFixedDelayModel()
	}

	// If the latency model is a FixedDelayModel, use its SetConnectionDelay
	// method
	if fdm, ok := n.latency.(*FixedDelayModel); ok {
		fdm.SetConnectionDelay(from, to, delay)
	}
}

// NewServer creates a new server on this P2P network with the given PeerId. The
// resulting server instance can be used by a node to interact with the network.
func (n *Network) NewServer(id PeerId) (Server, error) {
	// Check that the new server is not reusing an existing ID.
	if _, exists := n.peers[id]; exists {
		return nil, fmt.Errorf("server with ID %s already exists", id)
	}
	res := &server{
		id:       id,
		peers:    []PeerId{},
		handlers: []MessageHandler{},
		network:  n,
	}
	for otherId, peer := range n.peers {
		peer.connectTo(id)
		res.connectTo(otherId)
	}
	n.peers[id] = res
	return res, nil
}

// transferMessage sends a message from one peer to another within the network.
// The transfer will fail if either the sender or receiver is not connected to
// the network.
func (n *Network) transferMessage(from PeerId, to PeerId, msg Message) error {
	if _, exists := n.peers[from]; !exists {
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.peers[to]; !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}

	var delay time.Duration
	if n.latency != nil {
		delay = n.latency.GetDelay(from, to, msg)
	}

	n.tasks.Add(1)
	go func() {
		defer n.tasks.Done()
		if delay > 0 {
			time.Sleep(delay)
		}
		n.peers[to].receiveMessage(from, msg)
	}()
	return nil
}

// WaitForMessagesBeingDelivered blocks until all messages in the network have
// been delivered.
// This is a helper tool which prevents having to add sleep waits to guarantee
// delivery of asynchronous messages sent using [server.SendMessage]
//
// This function relies on synchronous message processing within each peer
// protocol and it needs to be called from the same goroutine that sends
// messages. Protocols implementing delayed message forwarding are not
// compatible with this function and other synchronization mechanisms shall be
// used to guarantee test completion.
//
// Protocols with unconstrained forwarding of messages in the network may lead
// to infinite wait time.
func (n *Network) WaitForDeliveryOfSentMessages() {
	n.tasks.Wait()
}
