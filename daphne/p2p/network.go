package p2p

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers      map[PeerId]peer
	tasks      sync.WaitGroup
	latency    LatencyModel
	tracker    tracker.Tracker
	msgCounter atomic.Uint64
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return NewNetworkWithLatency(nil)
}

// NewNetworkWithLatency creates a new P2P network with a custom latency model.
func NewNetworkWithLatency(model LatencyModel) *Network {
	return (&NetworkBuilder{}).WithLatency(model).Build()
}

// NetworkBuilder is a builder for creating a new P2P network with custom
// configurations.
type NetworkBuilder struct {
	latency LatencyModel
	tracker tracker.Tracker
}

// NewNetworkBuilder creates a new instance of NetworkBuilder creating a network
// with its default configuration.
func NewNetworkBuilder() *NetworkBuilder {
	return &NetworkBuilder{}
}

// WithLatency sets the latency model to be used by the network being built.
func (b *NetworkBuilder) WithLatency(latency LatencyModel) *NetworkBuilder {
	b.latency = latency
	return b
}

// WithTracker sets the event tracker to be used by the network being built.
func (b *NetworkBuilder) WithTracker(tracker tracker.Tracker) *NetworkBuilder {
	b.tracker = tracker
	return b
}

// Build constructs the Network instance from the builder's configuration.
func (b *NetworkBuilder) Build() *Network {
	return &Network{
		latency: b.latency,
		tracker: b.tracker,
		peers:   make(map[PeerId]peer),
	}
}

// NewServer creates a new server on this P2P network with the given PeerId. The
// resulting server instance can be used by a node to interact with the network.
// The new server will be automatically connected to all other servers already
// present in the network, and vice-versa. It will also be connected to itself.
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
	n.peers[id] = res
	for otherId, peer := range n.peers {
		peer.connectTo(id)
		res.connectTo(otherId)
	}
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

	id := n.msgCounter.Add(1)
	if n.tracker != nil {
		n.tracker.Track(mark.MsgSent, "id", id, "from", from, "to", to, "type", msg.Code)
	}

	n.tasks.Go(func() {
		if n.latency != nil {
			time.Sleep(n.latency.GetDelay(from, to, msg))
		}
		if n.tracker != nil {
			n.tracker.Track(mark.MsgReceived, "id", id, "from", from, "to", to, "type", msg.Code)
		}
		n.peers[to].receiveMessage(from, msg)
		if n.tracker != nil {
			n.tracker.Track(mark.MsgConsumed, "id", id, "from", from, "to", to, "type", msg.Code)
		}
	})
	return nil
}

// WaitForMessagesBeingDelivered blocks until all messages in the network have
// been delivered.
// This is a helper tool which prevents having to add sleep waits to guarantee
// delivery of asynchronous messages sent using [server.SendMessage]
//
// This function relies on synchronous message processing within each peer
// protocol and it needs to be called from the same goroutine that sends
// messages. Protocols that implement delayed message forwarding on top of
// asynchronous message processing at the network level, which in turn use
// their own logic to buffer and dispatch messages, are not compatible with this
// function. Other synchronization mechanisms would be required on top.
//
// Protocols with unconstrained forwarding of messages in the network may lead
// to infinite wait time.
func (n *Network) WaitForDeliveryOfSentMessages() {
	n.tasks.Wait()
}
